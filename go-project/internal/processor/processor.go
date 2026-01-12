package processor

import (
    "context"
    "sort"
    "strings"
    "sync"
    "time"

    "balance-service/internal/config"
	"balance-service/internal/model"
	"balance-service/internal/repository"
	"github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

const (
	batchTimeout = 5 * time.Second
	dbTimeout    = 10 * time.Second
)

// BalanceMessage represents the message format from RabbitMQ
type BalanceMessage struct {
	UserID    uint    `json:"user_id"`
	NewAmount float64 `json:"new_amount"` // PHP sends "new_amount"
	Amount    float64 `json:"amount"`     // Alternative field name
	Version   uint    `json:"version"`
	Timestamp string  `json:"timestamp"`   // ISO8601 format
	UpdatedAt string  `json:"updated_at"` // Alternative field name
	EventID   string  `json:"event_id"`
}

// GetAmount returns the amount value (handles both field names)
func (m *BalanceMessage) GetAmount() float64 {
	if m.NewAmount != 0 {
		return m.NewAmount
	}
	return m.Amount
}

// GetTimestamp returns the timestamp (handles both field names)
func (m *BalanceMessage) GetTimestamp() string {
	if m.UpdatedAt != "" {
		return m.UpdatedAt
	}
	return m.Timestamp
}

// ParseTimestamp parses the timestamp string to time.Time
func (m *BalanceMessage) ParseTimestamp() (time.Time, error) {
	ts := m.GetTimestamp()
	if ts == "" {
		return time.Now(), nil
	}

	// Try ISO8601 format
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		// Try other common formats
		formats := []string{
			"2006-01-02T15:04:05Z07:00",
			"2006-01-02 15:04:05",
			time.RFC3339Nano,
		}
		for _, format := range formats {
			if t, err := time.Parse(format, ts); err == nil {
				return t, nil
			}
		}
		return time.Now(), err
	}
	return t, nil
}

type IncomingUpdate struct {
	Payload  BalanceMessage
	Delivery amqp091.Delivery
}

func StartProcessorPool(
    ctx context.Context,
    balanceRepo *repository.BalanceRepository,
    eventRepo *repository.EventRepository,
    cache *sync.Map,
    updates <-chan IncomingUpdate,
    rabbitCfg config.RabbitConfig,
    batchSize int,
    log *logrus.Logger,
) {
//     numWorkers := runtime.NumCPU()
    numWorkers := rabbitCfg.Workers
    log.Infof("Starting processor pool with %d workers", numWorkers)

    for i := 0; i < numWorkers; i++ {
        go runWorker(ctx, i, balanceRepo, eventRepo, cache, updates, batchSize, log)
    }
}

func runWorker(
    ctx context.Context,
    id int,
    balanceRepo *repository.BalanceRepository,
    eventRepo *repository.EventRepository,
    cache *sync.Map,
    updates <-chan IncomingUpdate,
    batchSize int,
    log *logrus.Logger,
) {
    flushInterval := time.Duration(2000+(id*500)) * time.Millisecond
    ticker := time.NewTicker(flushInterval)
    defer ticker.Stop()

    batch := make([]IncomingUpdate, 0, batchSize)

    flush := func() {
        if len(batch) == 0 {
            return
        }
        localBatch := batch
        batch = make([]IncomingUpdate, 0, batchSize)

        maxRetries := 3
        var err error

        for i := 0; i < maxRetries; i++ {
            err = handleBatch(ctx, balanceRepo, eventRepo, cache, localBatch, log)
            if err == nil {
                break
            }

            if strings.Contains(err.Error(), "1213") || strings.Contains(err.Error(), "Deadlock") {
                log.Warnf("Worker %d: Deadlock detected (attempt %d/%d). Retrying...", id, i+1, maxRetries)
                time.Sleep(time.Millisecond * time.Duration(100*(i+1)))
                continue
            }
            break
        }

        if err != nil {
            log.Errorf("Worker %d fatal error after retries: %v", id, err)
            for _, upd := range localBatch {
                _ = upd.Delivery.Nack(false, true)
            }
        } else {
            for _, upd := range localBatch {
                _ = upd.Delivery.Ack(false)
            }
        }
    }

    for {
        select {
        case <-ctx.Done():
            flush()
            return
        case upd, ok := <-updates:
            if !ok {
                flush()
                return
            }
            batch = append(batch, upd)
            if len(batch) >= batchSize {
                flush()
            }
        case <-ticker.C:
            flush()
        }
    }
}

func handleBatch(
    ctx context.Context,
    balanceRepo *repository.BalanceRepository,
    eventRepo *repository.EventRepository,
    cache *sync.Map,
    updates []IncomingUpdate,
    log *logrus.Logger,
) error {
    ctx, cancel := context.WithTimeout(ctx, dbTimeout)
    defer cancel()

    deduped := make(map[uint]IncomingUpdate)
    events := make([]model.BalanceEvent, 0, len(updates))
    seenEventIDs := make(map[string]bool)

    for _, upd := range updates {
        payload := upd.Payload
        if payload.UserID == 0 {
            continue
        }

        if payload.EventID != "" {
            if !seenEventIDs[payload.EventID] {
                seenEventIDs[payload.EventID] = true
                ts, _ := payload.ParseTimestamp()
                events = append(events, model.BalanceEvent{
                    UserID:    payload.UserID,
                    Amount:    payload.GetAmount(),
                    Version:   payload.Version,
                    UpdatedAt: ts,
                    EventID:   payload.EventID,
                })
            }
        }

        existing, ok := deduped[payload.UserID]
        if !ok || payload.Version > existing.Payload.Version {
            deduped[payload.UserID] = upd
        }
    }

    if len(events) > 0 {
        if err := eventRepo.SaveEventsBatch(ctx, events); err != nil {
            return err
        }
    }

    balances := make([]model.Balance, 0, len(deduped))
    userIDs := make([]uint, 0, len(deduped))
    for _, upd := range deduped {
        balances = append(balances, model.Balance{
            UserID:  upd.Payload.UserID,
            Amount:  upd.Payload.GetAmount(),
            Version: upd.Payload.Version,
        })
        userIDs = append(userIDs, upd.Payload.UserID)
    }

    sort.Slice(balances, func(i, j int) bool {
        return balances[i].UserID < balances[j].UserID
    })

    if len(balances) > 0 {
        if err := balanceRepo.SaveBalancesBatch(ctx, balances); err != nil {
            return err
        }

        updatedBalances, err := balanceRepo.GetBalancesByUserIDs(ctx, userIDs)
        if err == nil {
            for _, b := range updatedBalances {
                cache.Store(b.UserID, b.Amount)
            }
        }

        log.WithFields(logrus.Fields{
            "balances": len(balances),
            "events":   len(events),
        }).Info("batch upsert committed")
    }

    return nil
}
