package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/alice101-dev/ai-code-reviewer/internal/database"
	"github.com/alice101-dev/ai-code-reviewer/internal/model"
)

type WalletRepository struct {
	db *gorm.DB
}

func NewWalletRepository(db *gorm.DB) *WalletRepository {
	return &WalletRepository{db: db}
}

func (r *WalletRepository) Create(ctx context.Context, wallet *model.Wallet) error {
	return translate(database.Conn(ctx, r.db).Create(wallet).Error)
}

func (r *WalletRepository) FindByIDForUser(ctx context.Context, id, userID uuid.UUID) (*model.Wallet, error) {
	var wallet model.Wallet
	err := database.Conn(ctx, r.db).
		Where("id = ? AND user_id = ?", id, userID).
		Take(&wallet).Error
	if err != nil {
		return nil, translate(err)
	}
	return &wallet, nil
}

func (r *WalletRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Wallet, error) {
	var wallets []model.Wallet
	err := database.Conn(ctx, r.db).
		Where("user_id = ?", userID).
		Order("created_at ASC, id ASC").
		Find(&wallets).Error
	if err != nil {
		return nil, translate(err)
	}
	return wallets, nil
}

func (r *WalletRepository) LockByIDs(ctx context.Context, ids ...uuid.UUID) ([]model.Wallet, error) {
	var wallets []model.Wallet
	err := database.Conn(ctx, r.db).
		Clauses(lockForUpdate()).
		Where("id IN ?", ids).
		Order("id ASC").
		Find(&wallets).Error
	if err != nil {
		return nil, translate(err)
	}
	return wallets, nil
}

func (r *WalletRepository) UpdateBalance(ctx context.Context, id uuid.UUID, balance int64) error {
	result := database.Conn(ctx, r.db).
		Model(&model.Wallet{}).
		Where("id = ?", id).
		Update("balance", balance)
	if result.Error != nil {
		return translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *WalletRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status model.WalletStatus) error {
	result := database.Conn(ctx, r.db).
		Model(&model.Wallet{}).
		Where("id = ?", id).
		Update("status", status)
	if result.Error != nil {
		return translate(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func lockForUpdate() clause.Locking {
	return clause.Locking{Strength: clause.LockingStrengthUpdate}
}
