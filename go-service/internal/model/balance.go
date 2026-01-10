package model

import (
	"time"
)

// Balance represents the current balance state for a user
type Balance struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	UserID    uint      `gorm:"uniqueIndex:idx_user_id;not null" json:"user_id"`
	Amount    float64   `gorm:"type:decimal(15,2);not null;default:0" json:"amount"`
	Version   uint      `gorm:"not null;default:0" json:"version"`
}

// TableName specifies the table name
func (Balance) TableName() string {
	return "balances"
}

// BalanceEvent represents a balance update event from RabbitMQ
type BalanceEvent struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UserID    uint      `gorm:"index:idx_user_id;index:idx_created_at;not null" json:"user_id"`
	Amount    float64   `gorm:"type:decimal(15,2);not null" json:"amount"`
	Version   uint      `gorm:"not null" json:"version"`
	UpdatedAt time.Time `gorm:"index:idx_created_at" json:"updated_at"`
	EventID   string    `gorm:"index:idx_event_id;size:255" json:"event_id"`
}

// TableName specifies the table name
func (BalanceEvent) TableName() string {
	return "balance_events"
}
