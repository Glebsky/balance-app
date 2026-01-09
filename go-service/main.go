package main

import (
	"context"
	"os/signal"
	"sync"
	"syscall"

	"balance-service/internal/config"
	"balance-service/internal/consumer"
	"balance-service/internal/database"
	"balance-service/internal/logger"
	"balance-service/internal/processor"
	"balance-service/internal/repository"
	cacheSync "balance-service/internal/sync"
)

var cache sync.Map

func main() {
	log := logger.New()
	cfg := config.Load()

	// Initialize database
	db, err := database.New(cfg.Database, log)
	if err != nil {
		log.WithError(err).Fatal("failed to initialize database")
	}
	sqlDB, _ := db.DB.DB()
	defer sqlDB.Close()

	// Initialize repositories
	balanceRepo := repository.NewBalanceRepository(db.DB, log)
	eventRepo := repository.NewEventRepository(db.DB, log)

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Create channel for incoming updates
	updates := make(chan processor.IncomingUpdate, cfg.Batch.Size*2)

	// Start processor goroutine
	go processor.ProcessBatches(
		ctx,
		balanceRepo,
		eventRepo,
		&cache,
		updates,
		cfg.Batch.Size,
		cfg.Batch.Interval,
		log,
	)

	// Start cache synchronizer goroutine
	go cacheSync.SyncCache(
		ctx,
		balanceRepo,
		&cache,
		cfg.Sync.BatchSize,
		cfg.Sync.Interval,
		log,
	)

	// Initialize and start RabbitMQ consumer
	rmqConsumer, err := consumer.New(cfg.Rabbit, log, updates)
	if err != nil {
		log.WithError(err).Fatal("failed to initialize RabbitMQ consumer")
	}
	defer rmqConsumer.Close()

	// Start consuming messages
	if err := rmqConsumer.Start(ctx); err != nil && ctx.Err() == nil {
		log.WithError(err).Fatal("consumer stopped unexpectedly")
	}

	log.Info("graceful shutdown complete")
}
