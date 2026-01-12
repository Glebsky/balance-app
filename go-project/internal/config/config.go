package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Database DatabaseConfig
	Rabbit   RabbitConfig
	Batch    BatchConfig
	Sync     SyncConfig
}

type DatabaseConfig struct {
    Host     string
    Port     int
    User     string
    Password string
    DBName   string
}

type RabbitConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	VHost    string
	Queue    string
	Prefetch int
	Workers  int
}

type BatchConfig struct {
	Size     int
	Interval time.Duration
}

type SyncConfig struct {
	Interval  time.Duration
	BatchSize int
}

func Load() *Config {
// 	dbPort := intFromEnv("DB_PORT", 5432)
	rmqPort := intFromEnv("RABBITMQ_PORT", 5672)

	return &Config{
		Database: DatabaseConfig{
			Host:     getenv("DB_HOST", "mysql-go"),
			Port:     intFromEnv("DB_PORT", 3306),
			User:     getenv("DB_USER", getenv("DB_USERNAME", "go")),
			Password: getenv("DB_PASSWORD", "go"),
			DBName:   getenv("DB_NAME", getenv("DB_DATABASE", "go_db")),
		},
		Rabbit: RabbitConfig{
			Host:     getenv("RABBITMQ_HOST", "localhost"),
			Port:     rmqPort,
			User:     getenv("RABBITMQ_USER", "guest"),
			Password: getenv("RABBITMQ_PASSWORD", "guest"),
			VHost:    getenv("RABBITMQ_VHOST", "/"),
			Queue:    getenv("RABBITMQ_QUEUE", "balance_updates"),
			Prefetch: intFromEnv("RABBITMQ_PREFETCH", 50),
			Workers:  clamp(intFromEnv("RABBITMQ_WORKERS", 5), 5, 10),
		},
		Batch: BatchConfig{
			Size:     intFromEnv("BATCH_SIZE", 100),
			Interval: time.Duration(intFromEnv("BATCH_INTERVAL_SECONDS", 5)) * time.Second,
		},
		Sync: SyncConfig{
			Interval:  time.Duration(intFromEnv("SYNC_INTERVAL_SECONDS", 30)) * time.Second,
			BatchSize: intFromEnv("SYNC_BATCH_SIZE", 1000),
		},
	}
}

func getenv(key, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}

	return def
}

func intFromEnv(key string, def int) int {
	val := getenv(key, "")
	if val == "" {
		return def
	}

	if parsed, err := strconv.Atoi(val); err == nil {
		return parsed
	}

	return def
}

func clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

