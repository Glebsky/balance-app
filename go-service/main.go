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
	"github.com/sirupsen/logrus"
)

var cache sync.Map

func main() {
	log := logger.New()
	cfg := config.Load()

	log.WithFields(logrus.Fields{
		"rabbitmq_queue": cfg.Rabbit.Queue,
		"rabbitmq_host":  cfg.Rabbit.Host,
		"db_host":        cfg.Database.Host,
		"db_name":        cfg.Database.DBName,
		"workers":        cfg.Rabbit.Workers,
		"batch_size":     cfg.Batch.Size,
	}).Info("starting balance service")

	// Initialize database
	db, err := database.New(cfg.Database, log)
	if err != nil {
		log.WithError(err).Fatal("failed to initialize database")
	}
	sqlDB, err := db.DB.DB()
	if err != nil {
		log.WithError(err).Fatal("failed to get database connection")
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			log.WithError(err).Error("error closing database connection")
		}
	}()

	// Initialize repositories
	balanceRepo := repository.NewBalanceRepository(db.DB, log)
	eventRepo := repository.NewEventRepository(db.DB, log)

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Create channel for incoming updates (buffered to handle bursts)
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
	log.Info("batch processor started")

	// Start cache synchronizer goroutine
	go cacheSync.SyncCache(
		ctx,
		balanceRepo,
		&cache,
		cfg.Sync.BatchSize,
		cfg.Sync.Interval,
		log,
	)
	log.Info("cache synchronizer started")

	// Initialize and start RabbitMQ consumer
	rmqConsumer, err := consumer.New(cfg.Rabbit, log, updates)
	if err != nil {
		log.WithError(err).Fatal("failed to initialize RabbitMQ consumer")
	}
	defer func() {
		log.Info("closing RabbitMQ consumer")
		rmqConsumer.Close()
	}()

	log.Info("balance service started, waiting for messages...")

	// Start consuming messages (this blocks until context is cancelled)
	if err := rmqConsumer.Start(ctx); err != nil && ctx.Err() == nil {
		log.WithError(err).Fatal("consumer stopped unexpectedly")
	}

	log.Info("graceful shutdown complete")
}
