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

// SyncCache periodically refreshes the local cache from PostgreSQL in batches
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

	total, err := balanceRepo.CountBalances(ctx)
	if err != nil {
		log.WithError(err).Error("failed to count balances for cache sync")
		return
	}

	if total == 0 {
		log.Debug("no balances to sync")
		return
	}

	log.WithField("total", total).Debug("starting cache synchronization")

	var synced int64
	offset := 0

	for {
		balances, err := balanceRepo.GetAllBalances(ctx, batchSize, offset)
		if err != nil {
			log.WithError(err).Error("failed to fetch balances batch")
			break
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

		// Check if we've processed all records
		if len(balances) < batchSize {
			break
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			log.Info("cache sync cancelled")
			return
		default:
		}
	}

	log.WithFields(logrus.Fields{
		"synced": synced,
		"total":  total,
	}).Info("cache synchronization completed")
}
