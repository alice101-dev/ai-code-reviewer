package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/alice101-dev/ai-code-reviewer/internal/database"
	"github.com/alice101-dev/ai-code-reviewer/internal/model"
)

type TransactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

func (r *TransactionRepository) Create(ctx context.Context, txn *model.Transaction) error {
	return translate(database.Conn(ctx, r.db).Create(txn).Error)
}

func (r *TransactionRepository) FindByIdempotencyKey(ctx context.Context, userID uuid.UUID, key string) (*model.Transaction, error) {
	var txn model.Transaction
	err := database.Conn(ctx, r.db).
		Where("user_id = ? AND idempotency_key = ?", userID, key).
		Take(&txn).Error
	if err != nil {
		return nil, translate(err)
	}
	return &txn, nil
}

func (r *TransactionRepository) CreateEntries(ctx context.Context, entries []model.LedgerEntry) error {
	return translate(database.Conn(ctx, r.db).Create(&entries).Error)
}

func (r *TransactionRepository) ListWalletActivity(ctx context.Context, walletID uuid.UUID, limit, offset int) ([]model.WalletActivity, error) {
	var activities []model.WalletActivity
	err := database.Conn(ctx, r.db).
		Table("ledger_entries AS le").
		Select(`le.id AS entry_id,
			le.transaction_id,
			t.type,
			le.direction,
			le.amount,
			le.balance_after,
			t.currency,
			t.description,
			CASE WHEN le.direction = 'debit' THEN t.destination_wallet_id ELSE t.source_wallet_id END AS counterparty_wallet_id,
			le.created_at`).
		Joins("JOIN transactions AS t ON t.id = le.transaction_id").
		Where("le.wallet_id = ?", walletID).
		Order("le.created_at DESC, le.id DESC").
		Limit(limit).
		Offset(offset).
		Scan(&activities).Error
	if err != nil {
		return nil, translate(err)
	}
	return activities, nil
}

func (r *TransactionRepository) CountWalletActivity(ctx context.Context, walletID uuid.UUID) (int64, error) {
	var total int64
	err := database.Conn(ctx, r.db).
		Model(&model.LedgerEntry{}).
		Where("wallet_id = ?", walletID).
		Count(&total).Error
	if err != nil {
		return 0, translate(err)
	}
	return total, nil
}
