package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Database DatabaseConfig
	RabbitMQ RabbitMQConfig
	Sync     SyncConfig
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type RabbitMQConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	VHost    string
	Exchange string
	Queue    string
}

type SyncConfig struct {
	Interval time.Duration
	BatchSize int
}

func Load() (*Config, error) {
	dbPort, _ := strconv.Atoi(getEnv("DB_PORT", "5432"))
	rmqPort, _ := strconv.Atoi(getEnv("RABBITMQ_PORT", "5672"))
	syncInterval, _ := strconv.Atoi(getEnv("SYNC_INTERVAL_SECONDS", "30"))
	batchSize, _ := strconv.Atoi(getEnv("SYNC_BATCH_SIZE", "100"))

	return &Config{
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     dbPort,
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			DBName:   getEnv("DB_NAME", "balance_db"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		RabbitMQ: RabbitMQConfig{
			Host:     getEnv("RABBITMQ_HOST", "localhost"),
			Port:     rmqPort,
			User:     getEnv("RABBITMQ_USER", "guest"),
			Password: getEnv("RABBITMQ_PASSWORD", "guest"),
			VHost:    getEnv("RABBITMQ_VHOST", "/"),
			Exchange: getEnv("RABBITMQ_EXCHANGE", "balance_exchange"),
			Queue:    getEnv("RABBITMQ_QUEUE", "balance_updates"),
		},
		Sync: SyncConfig{
			Interval:  time.Duration(syncInterval) * time.Second,
			BatchSize: batchSize,
		},
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

