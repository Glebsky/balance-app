package sync

import (
	"context"
	"sync"
	"time"

	"balance-service/internal/repository"
	"github.com/sirupsen/logrus"
)

const (
	syncTimeout = 30 * time.Second
)

// SyncCache periodically refreshes the local cache from MySql in batches
func SyncCache(
	ctx context.Context,
	balanceRepo *repository.BalanceRepository,
	cache *sync.Map,
	batchSize int,
	interval time.Duration,
	log *logrus.Logger,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run initial sync
	runSync(ctx, balanceRepo, cache, batchSize, log)

	for {
		select {
		case <-ctx.Done():
			log.Info("stopping cache synchronizer")
			return
		case <-ticker.C:
			runSync(ctx, balanceRepo, cache, batchSize, log)
		}
	}
}

func runSync(
	ctx context.Context,
	balanceRepo *repository.BalanceRepository,
	cache *sync.Map,
	batchSize int,
	log *logrus.Logger,
) {
	ctx, cancel := context.WithTimeout(ctx, syncTimeout)
	defer cancel()

	startTime := time.Now()

	total, err := balanceRepo.CountBalances(ctx)
	if err != nil {
		log.WithError(err).Error("failed to count balances for cache sync")
		return
	}

	if total == 0 {
		log.Debug("no balances to sync")
		return
	}

	log.WithField("total", total).Info("starting cache synchronization")

	var synced int64
	offset := 0
	errors := 0
	maxErrors := 3

	for {
		// Check context cancellation before each batch
		select {
		case <-ctx.Done():
			log.Info("cache sync cancelled")
			return
		default:
		}

		balances, err := balanceRepo.GetAllBalances(ctx, batchSize, offset)
		if err != nil {
			errors++
			log.WithFields(logrus.Fields{
				"error":   err,
				"offset":  offset,
				"errors":  errors,
			}).Error("failed to fetch balances batch")

			if errors >= maxErrors {
				log.Error("max errors reached, stopping cache sync")
				break
			}

			// Wait a bit before retrying
			select {
			case <-time.After(time.Second):
			case <-ctx.Done():
				return
			}
			continue
		}

		if len(balances) == 0 {
			break
		}

		// Update cache safely
		for _, b := range balances {
			cache.Store(b.UserID, b.Amount)
			synced++
		}

		offset += len(balances)
		errors = 0 // Reset error counter on success

		// Check if we've processed all records
		if len(balances) < batchSize {
			break
		}
	}

	duration := time.Since(startTime)
	log.WithFields(logrus.Fields{
		"synced":   synced,
		"total":    total,
		"duration": duration,
		"rate":     float64(synced) / duration.Seconds(),
	}).Info("cache synchronization completed")
}
