package database

import (
	"balance-consumer/internal/config"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

type DB struct {
	pool *pgxpool.Pool
	log  *logrus.Logger
}

func New(cfg config.DatabaseConfig) (*DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode,
	)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	poolConfig.MaxConns = 25
	poolConfig.MinConns = 5
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = time.Minute * 30

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log := logrus.New()
	log.Info("Database connection pool created")

	db := &DB{
		pool: pool,
		log:  log,
	}

	// Initialize schema
	if err := db.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

func (db *DB) initSchema() error {
	query := `
		CREATE TABLE IF NOT EXISTS balance_updates (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL,
			old_amount DECIMAL(15,2) NOT NULL,
			new_amount DECIMAL(15,2) NOT NULL,
			version INTEGER NOT NULL,
			event_id VARCHAR(255) NOT NULL,
			timestamp TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(event_id),
			UNIQUE(user_id, version)
		);

		CREATE INDEX IF NOT EXISTS idx_balance_updates_user_id ON balance_updates(user_id);
		CREATE INDEX IF NOT EXISTS idx_balance_updates_timestamp ON balance_updates(timestamp);
		CREATE INDEX IF NOT EXISTS idx_balance_updates_event_id ON balance_updates(event_id);
	`

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := db.pool.Exec(ctx, query)
	return err
}

func (db *DB) SaveBalanceUpdate(ctx context.Context, userID int, oldAmount, newAmount float64, version int, eventID string, timestamp time.Time) error {
	// Check if event_id already exists (idempotency)
	checkQuery := `SELECT id FROM balance_updates WHERE event_id = $1 LIMIT 1`
	var existingID int
	err := db.pool.QueryRow(ctx, checkQuery, eventID).Scan(&existingID)
	if err == nil {
		// Event already processed, skip (idempotency)
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		// Real error occurred
		return fmt.Errorf("failed to check event_id: %w", err)
	}

	// Try to insert, handle conflicts
	query := `
		INSERT INTO balance_updates (user_id, old_amount, new_amount, version, event_id, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, version) DO UPDATE SET
			new_amount = CASE 
				WHEN balance_updates.timestamp < EXCLUDED.timestamp THEN EXCLUDED.new_amount
				ELSE balance_updates.new_amount
			END,
			timestamp = CASE 
				WHEN balance_updates.timestamp < EXCLUDED.timestamp THEN EXCLUDED.timestamp
				ELSE balance_updates.timestamp
			END,
			event_id = CASE 
				WHEN balance_updates.timestamp < EXCLUDED.timestamp THEN EXCLUDED.event_id
				ELSE balance_updates.event_id
			END
	`
	
	_, err = db.pool.Exec(ctx, query, userID, oldAmount, newAmount, version, eventID, timestamp)
	if err != nil {
		return fmt.Errorf("failed to save balance update: %w", err)
	}

	return nil
}

func (db *DB) GetLatestBalances(ctx context.Context, limit int) (map[int]BalanceData, error) {
	// Use window function for better compatibility
	query := `
		SELECT user_id, new_amount, version, timestamp
		FROM (
			SELECT user_id, new_amount, version, timestamp,
				   ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY version DESC) as rn
			FROM balance_updates
		) ranked
		WHERE rn = 1
		LIMIT $1
	`

	rows, err := db.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query latest balances: %w", err)
	}
	defer rows.Close()

	balances := make(map[int]BalanceData)
	for rows.Next() {
		var data BalanceData
		if err := rows.Scan(&data.UserID, &data.Amount, &data.Version, &data.Timestamp); err != nil {
			return nil, fmt.Errorf("failed to scan balance: %w", err)
		}
		balances[data.UserID] = data
	}

	return balances, rows.Err()
}

func (db *DB) Close() {
	db.pool.Close()
}

type BalanceData struct {
	UserID    int
	Amount    float64
	Version   int
	Timestamp time.Time
}

