package repository

import (
	"context"

	"balance-service/internal/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type EventRepository struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewEventRepository(db *gorm.DB, log *logrus.Logger) *EventRepository {
	return &EventRepository{
		db:  db,
		log: log,
	}
}

// SaveEvent saves a balance event
func (r *EventRepository) SaveEvent(ctx context.Context, event *model.BalanceEvent) error {
	return r.db.WithContext(ctx).Create(event).Error
}

func (r *EventRepository) SaveEventsBatch(ctx context.Context, events []model.BalanceEvent) error {
    if len(events) == 0 {
        return nil
    }

    return r.db.WithContext(ctx).
        Clauses(clause.OnConflict{
            Columns:   []clause.Column{{Name: "event_id"}}, // Укажи здесь имя своего уникального индекса
            DoNothing: true,
        }).
        Create(&events).
        Error
}

// EventExists checks if an event with given event_id already exists
func (r *EventRepository) EventExists(ctx context.Context, eventID string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.BalanceEvent{}).
		Where("event_id = ?", eventID).
		Count(&count).Error

	return count > 0, err
}
