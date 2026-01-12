package database

import (
	"fmt"
	"time"

	"balance-service/internal/config"
	"balance-service/internal/model"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Database struct {
	DB *gorm.DB
}

func New(cfg config.DatabaseConfig, log *logrus.Logger) (*Database, error) {
	// MySQL DSN format: user:password@tcp(host:port)/dbname?parseTime=true
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName,
	)
    dsn = "go:go@tcp(mysql:3306)/go_db?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci"

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
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
		"port": cfg.Port,
		"db":   cfg.DBName,
	}).Info("connected to MySQL")

	return &Database{DB: db}, nil
}
