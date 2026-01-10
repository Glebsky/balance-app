package repository

import (
	"context"

	"balance-service/internal/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BalanceRepository struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewBalanceRepository(db *gorm.DB, log *logrus.Logger) *BalanceRepository {
	return &BalanceRepository{
		db:  db,
		log: log,
	}
}

// SaveBalance saves or updates a balance record
func (r *BalanceRepository) SaveBalance(ctx context.Context, balance *model.Balance) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"amount": gorm.Expr("CASE WHEN balances.version <= VALUES(version) THEN VALUES(amount) ELSE balances.amount END"),
			"version": gorm.Expr("GREATEST(balances.version, VALUES(version))"),
			"updated_at": gorm.Expr("NOW()"),
		}),
	}).Create(balance).Error
}

// SaveBalancesBatch saves multiple balances in a batch
func (r *BalanceRepository) SaveBalancesBatch(ctx context.Context, balances []model.Balance) error {
	if len(balances) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"amount": gorm.Expr("CASE WHEN balances.version <= VALUES(version) THEN VALUES(amount) ELSE balances.amount END"),
			"version": gorm.Expr("GREATEST(balances.version, VALUES(version))"),
			"updated_at": gorm.Expr("NOW()"),
		}),
	}).Create(&balances).Error
}

// GetBalancesByUserIDs retrieves balances for given user IDs
func (r *BalanceRepository) GetBalancesByUserIDs(ctx context.Context, userIDs []uint) ([]model.Balance, error) {
	var balances []model.Balance
	if len(userIDs) == 0 {
		return balances, nil
	}

	err := r.db.WithContext(ctx).
		Where("user_id IN ?", userIDs).
		Find(&balances).Error

	return balances, err
}

// GetAllBalances retrieves all balances (for cache sync)
func (r *BalanceRepository) GetAllBalances(ctx context.Context, limit, offset int) ([]model.Balance, error) {
	var balances []model.Balance
	err := r.db.WithContext(ctx).
		Select("user_id", "amount", "version").
		Limit(limit).
		Offset(offset).
		Find(&balances).Error

	return balances, err
}

// CountBalances returns total count of balances
func (r *BalanceRepository) CountBalances(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Balance{}).Count(&count).Error
	return count, err
}
