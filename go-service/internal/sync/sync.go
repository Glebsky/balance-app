package sync

import (
	"balance-consumer/internal/config"
	"balance-consumer/internal/database"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type BalanceCache struct {
	Amount    float64
	Version   int
	Timestamp time.Time
}

type CacheSynchronizer struct {
	cache     sync.Map // map[int]*BalanceCache
	db        *database.DB
	config    config.SyncConfig
	log       *logrus.Logger
	mu        sync.RWMutex
}

func NewCacheSynchronizer(db *database.DB, cfg config.SyncConfig, log *logrus.Logger) *CacheSynchronizer {
	return &CacheSynchronizer{
		db:     db,
		config: cfg,
		log:    log,
	}
}

func (cs *CacheSynchronizer) UpdateCache(userID int, amount float64, version int, timestamp time.Time) {
	cs.cache.Store(userID, &BalanceCache{
		Amount:    amount,
		Version:   version,
		Timestamp: timestamp,
	})
}

func (cs *CacheSynchronizer) GetCache(userID int) (*BalanceCache, bool) {
	value, ok := cs.cache.Load(userID)
	if !ok {
		return nil, false
	}
	return value.(*BalanceCache), true
}

func (cs *CacheSynchronizer) Start(ctx context.Context) {
	cs.log.Info("Starting cache synchronizer")

	ticker := time.NewTicker(cs.config.Interval)
	defer ticker.Stop()

	// Initial sync
	cs.syncCache()

	for {
		select {
		case <-ctx.Done():
			cs.log.Info("Stopping cache synchronizer")
			return
		case <-ticker.C:
			cs.syncCache()
		}
	}
}

func (cs *CacheSynchronizer) syncCache() {
	cs.log.Debug("Starting cache synchronization")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get latest balances from database
	dbBalances, err := cs.db.GetLatestBalances(ctx, cs.config.BatchSize)
	if err != nil {
		cs.log.WithError(err).Error("Failed to get latest balances from database")
		return
	}

	// Update cache with database values
	updated := 0
	conflicts := 0

	for userID, dbBalance := range dbBalances {
		cachedValue, exists := cs.cache.Load(userID)
		
		if !exists {
			// New entry, add to cache
			cs.UpdateCache(userID, dbBalance.Amount, dbBalance.Version, dbBalance.Timestamp)
			updated++
		} else {
			cached := cachedValue.(*BalanceCache)
			// Check for conflicts - database version should be >= cache version
			if dbBalance.Version > cached.Version {
				// Database is newer, update cache
				cs.UpdateCache(userID, dbBalance.Amount, dbBalance.Version, dbBalance.Timestamp)
				updated++
			} else if dbBalance.Version < cached.Version {
				// Cache is newer, this shouldn't happen but log it
				conflicts++
				cs.log.WithFields(logrus.Fields{
					"user_id": userID,
					"cache_version": cached.Version,
					"db_version": dbBalance.Version,
				}).Warn("Version conflict detected")
			}
		}
	}

	cs.log.WithFields(logrus.Fields{
		"updated": updated,
		"conflicts": conflicts,
		"total": len(dbBalances),
	}).Info("Cache synchronization completed")
}

