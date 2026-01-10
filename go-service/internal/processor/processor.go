package processor

import (
	"context"
	"sync"
	"time"

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

// ProcessBatches accumulates incoming updates and writes them to db in batches
func ProcessBatches(
	ctx context.Context,
	balanceRepo *repository.BalanceRepository,
	eventRepo *repository.EventRepository,
	cache *sync.Map,
	updates <-chan IncomingUpdate,
	batchSize int,
	flushInterval time.Duration,
	log *logrus.Logger,
) {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	batch := make([]IncomingUpdate, 0, batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}

		local := batch
		batch = make([]IncomingUpdate, 0, batchSize)

		log.WithField("batch_size", len(local)).Debug("processing batch")

		if err := handleBatch(ctx, balanceRepo, eventRepo, cache, local, log); err != nil {
			log.WithFields(logrus.Fields{
				"error":      err,
				"batch_size": len(local),
			}).Error("failed to process batch, nacking messages for retry")
			// Nack all messages in batch for retry
			for _, upd := range local {
				if err := upd.Delivery.Nack(false, true); err != nil {
					log.WithError(err).Warn("failed to nack message")
				}
			}
			return
		}

		// Ack all remaining messages after successful processing
		acked := 0
		for _, upd := range local {
			if err := upd.Delivery.Ack(false); err != nil {
				log.WithError(err).Warn("failed to ack message")
			} else {
				acked++
			}
		}

		log.WithFields(logrus.Fields{
			"total": len(local),
			"acked": acked,
		}).Debug("batch processed successfully")
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

	// Deduplicate by user_id, keeping the highest version
	deduped := make(map[uint]IncomingUpdate)
	events := make([]model.BalanceEvent, 0, len(updates))
	seenEventIDs := make(map[string]bool)
	toAck := make([]amqp091.Delivery, 0, len(updates)) // Messages to ack immediately (duplicates)

	for _, upd := range updates {
		payload := upd.Payload

		// Validate payload
		if payload.UserID == 0 {
			log.WithField("payload", payload).Warn("invalid user_id in message, skipping")
			_ = upd.Delivery.Nack(false, false) // Don't requeue invalid messages
			continue
		}

		// Check for duplicate event_id in batch
		if payload.EventID != "" {
			if seenEventIDs[payload.EventID] {
				log.WithFields(logrus.Fields{
					"event_id": payload.EventID,
					"user_id":  payload.UserID,
				}).Debug("duplicate event_id in batch, skipping")
				toAck = append(toAck, upd.Delivery) // Ack duplicate in batch
				continue
			}
			seenEventIDs[payload.EventID] = true
		}

		// Check if event already exists in DB (if event_id provided)
		if payload.EventID != "" {
			exists, err := eventRepo.EventExists(ctx, payload.EventID)
			if err != nil {
				log.WithError(err).Warn("failed to check event existence, will process anyway")
			} else if exists {
				log.WithFields(logrus.Fields{
					"event_id": payload.EventID,
					"user_id":  payload.UserID,
				}).Debug("event already exists in DB, skipping")
				toAck = append(toAck, upd.Delivery) // Ack already processed message
				continue
			}
		}

		// Keep the highest version for each user
		existing, ok := deduped[payload.UserID]
		if !ok || payload.Version > existing.Payload.Version {
			deduped[payload.UserID] = upd
		}

		// Prepare event record
		timestamp, err := payload.ParseTimestamp()
		if err != nil {
			log.WithFields(logrus.Fields{
				"error":     err,
				"timestamp": payload.GetTimestamp(),
				"user_id":   payload.UserID,
			}).Warn("failed to parse timestamp, using current time")
			timestamp = time.Now()
		}

		event := model.BalanceEvent{
			UserID:    payload.UserID,
			Amount:    payload.GetAmount(),
			Version:   payload.Version,
			UpdatedAt: timestamp,
			EventID:   payload.EventID,
		}
		events = append(events, event)
	}

	// Ack duplicate/already-processed messages immediately
	for _, delivery := range toAck {
		if err := delivery.Ack(false); err != nil {
			log.WithError(err).Warn("failed to ack duplicate message")
		}
	}

	// Save events first
	if len(events) > 0 {
		if err := eventRepo.SaveEventsBatch(ctx, events); err != nil {
			return err
		}
		log.WithField("count", len(events)).Debug("events saved")
	}

	// Prepare balance records
	balances := make([]model.Balance, 0, len(deduped))
	for _, upd := range deduped {
		balances = append(balances, model.Balance{
			UserID:  upd.Payload.UserID,
			Amount:  upd.Payload.GetAmount(),
			Version: upd.Payload.Version,
		})
	}

	// Save balances
	if len(balances) > 0 {
		if err := balanceRepo.SaveBalancesBatch(ctx, balances); err != nil {
			return err
		}

		// Update cache with latest balances
		userIDs := make([]uint, 0, len(balances))
		for _, b := range balances {
			userIDs = append(userIDs, b.UserID)
		}

		// Fetch updated balances to ensure cache has correct data
		updatedBalances, err := balanceRepo.GetBalancesByUserIDs(ctx, userIDs)
		if err != nil {
			log.WithError(err).Warn("failed to fetch updated balances for cache")
		} else {
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
