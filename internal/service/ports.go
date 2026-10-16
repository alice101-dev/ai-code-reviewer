package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/alice101-dev/ai-code-reviewer/internal/model"
)

type TxRunner interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type UserStore interface {
	Create(ctx context.Context, user *model.User) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	FindByEmail(ctx context.Context, email string) (*model.User, error)
}

type RefreshTokenStore interface {
	Create(ctx context.Context, token *model.RefreshToken) error
	FindByHashForUpdate(ctx context.Context, hash string) (*model.RefreshToken, error)
	Revoke(ctx context.Context, id uuid.UUID, at time.Time) error
	RevokeByHash(ctx context.Context, userID uuid.UUID, hash string, at time.Time) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID, at time.Time) error
}

type WalletStore interface {
	Create(ctx context.Context, wallet *model.Wallet) error
	FindByIDForUser(ctx context.Context, id, userID uuid.UUID) (*model.Wallet, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Wallet, error)
	LockByIDs(ctx context.Context, ids ...uuid.UUID) ([]model.Wallet, error)
	UpdateBalance(ctx context.Context, id uuid.UUID, balance int64) error
}

type TransactionStore interface {
	Create(ctx context.Context, txn *model.Transaction) error
	FindByIdempotencyKey(ctx context.Context, userID uuid.UUID, key string) (*model.Transaction, error)
	CreateEntries(ctx context.Context, entries []model.LedgerEntry) error
	ListWalletActivity(ctx context.Context, walletID uuid.UUID, filter model.ActivityFilter, limit, offset int) ([]model.WalletActivity, error)
	CountWalletActivity(ctx context.Context, walletID uuid.UUID, filter model.ActivityFilter) (int64, error)
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(hash, password string) (bool, error)
}

type AccessTokenIssuer interface {
	Issue(userID uuid.UUID) (string, time.Time, error)
}
