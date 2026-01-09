package database

import (
	"fmt"
	"time"

	"balance-service/internal/config"
	"balance-service/internal/model"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Database struct {
	DB *gorm.DB
}

func New(cfg config.DatabaseConfig, log *logrus.Logger) (*Database, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := db.AutoMigrate(&model.Balance{}, &model.BalanceEvent{}); err != nil {
		return nil, fmt.Errorf("failed to migrate schema: %w", err)
	}

	log.WithFields(logrus.Fields{
		"host": cfg.Host,
		"db":   cfg.DBName,
	}).Info("connected to PostgreSQL")

	return &Database{DB: db}, nil
}

