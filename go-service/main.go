package main

import (
	"balance-consumer/internal/config"
	"balance-consumer/internal/consumer"
	"balance-consumer/internal/database"
	"balance-consumer/internal/logger"
	"balance-consumer/internal/sync"
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Initialize logger
	log := logger.New()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.WithError(err).Fatal("Failed to load configuration")
	}

	// Initialize database
	db, err := database.New(cfg.Database)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize database")
	}
	defer db.Close()

	log.Info("Database connection established")

	// Initialize cache synchronizer
	cacheSync := sync.NewCacheSynchronizer(db, cfg.Sync, log)
	
	// Start cache synchronization in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go cacheSync.Start(ctx)

	// Initialize RabbitMQ consumer
	rmqConsumer, err := consumer.New(cfg.RabbitMQ, db, cacheSync, log)
	if err != nil {
		log.WithError(err).Fatal("Failed to initialize RabbitMQ consumer")
	}
	defer rmqConsumer.Close()

	log.Info("RabbitMQ consumer initialized")

	// Start consuming messages
	go func() {
		if err := rmqConsumer.Start(); err != nil {
			log.WithError(err).Fatal("Failed to start consumer")
		}
	}()

	log.Info("Balance consumer service started")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down...")
	cancel()
	time.Sleep(2 * time.Second) // Give time for graceful shutdown
}

