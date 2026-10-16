package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/alice101-dev/ai-code-reviewer/internal/database"
	"github.com/alice101-dev/ai-code-reviewer/internal/model"
)

const walletActivityColumns = `le.id AS entry_id,
	le.transaction_id,
	t.type,
	le.direction,
	le.amount,
	le.balance_after,
	t.currency,
	t.description,
	CASE WHEN le.direction = 'debit' THEN t.destination_wallet_id ELSE t.source_wallet_id END AS counterparty_wallet_id,
	le.created_at`

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

func (r *TransactionRepository) ListWalletActivity(ctx context.Context, walletID uuid.UUID, filter model.ActivityFilter, limit, offset int) ([]model.WalletActivity, error) {
	var activities []model.WalletActivity
	err := database.Conn(ctx, r.db).
		Scopes(walletActivityScope(walletID, filter)).
		Select(walletActivityColumns).
		Order("le.created_at DESC, le.id DESC").
		Limit(limit).
		Offset(offset).
		Scan(&activities).Error
	if err != nil {
		return nil, translate(err)
	}
	return activities, nil
}

func (r *TransactionRepository) ListWalletActivityBefore(ctx context.Context, walletID uuid.UUID, filter model.ActivityFilter, before *model.ActivityCursor, limit int) ([]model.WalletActivity, error) {
	query := database.Conn(ctx, r.db).
		Scopes(walletActivityScope(walletID, filter)).
		Select(walletActivityColumns)
	if before != nil {
		query = query.Where("(le.created_at, le.id) < (?, ?)", before.CreatedAt, before.EntryID)
	}

	var activities []model.WalletActivity
	err := query.
		Order("le.created_at DESC, le.id DESC").
		Limit(limit).
		Scan(&activities).Error
	if err != nil {
		return nil, translate(err)
	}
	return activities, nil
}

func (r *TransactionRepository) CountWalletActivity(ctx context.Context, walletID uuid.UUID, filter model.ActivityFilter) (int64, error) {
	var total int64
	err := database.Conn(ctx, r.db).
		Scopes(walletActivityScope(walletID, filter)).
		Count(&total).Error
	if err != nil {
		return 0, translate(err)
	}
	return total, nil
}

func (r *TransactionRepository) SummarizeWalletActivity(ctx context.Context, walletID uuid.UUID, filter model.ActivityFilter) (model.WalletSummary, error) {
	var row struct {
		Credits int64
		Debits  int64
		Count   int64
	}
	err := database.Conn(ctx, r.db).
		Scopes(walletActivityScope(walletID, filter)).
		Select(`COALESCE(SUM(CASE WHEN le.direction = 'credit' THEN le.amount ELSE 0 END), 0) AS credits,
			COALESCE(SUM(CASE WHEN le.direction = 'debit' THEN le.amount ELSE 0 END), 0) AS debits,
			COUNT(*) AS count`).
		Scan(&row).Error
	if err != nil {
		return model.WalletSummary{}, translate(err)
	}

	return model.WalletSummary{
		TotalCredits:     row.Credits,
		TotalDebits:      row.Debits,
		TransactionCount: row.Count,
	}, nil
}

func walletActivityScope(walletID uuid.UUID, filter model.ActivityFilter) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		db = db.Table("ledger_entries AS le").
			Joins("JOIN transactions AS t ON t.id = le.transaction_id").
			Where("le.wallet_id = ?", walletID)

		if filter.Type != "" {
			db = db.Where("t.type = ?", filter.Type)
		}
		if filter.Direction != "" {
			db = db.Where("le.direction = ?", filter.Direction)
		}
		if !filter.From.IsZero() {
			db = db.Where("le.created_at >= ?", filter.From)
		}
		if !filter.To.IsZero() {
			db = db.Where("le.created_at < ?", filter.To)
		}

		return db
	}
}
