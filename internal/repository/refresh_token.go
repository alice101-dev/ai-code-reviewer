package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/alice101-dev/ai-code-reviewer/internal/database"
	"github.com/alice101-dev/ai-code-reviewer/internal/model"
)

type RefreshTokenRepository struct {
	db *gorm.DB
}

func NewRefreshTokenRepository(db *gorm.DB) *RefreshTokenRepository {
	return &RefreshTokenRepository{db: db}
}

func (r *RefreshTokenRepository) Create(ctx context.Context, token *model.RefreshToken) error {
	return translate(database.Conn(ctx, r.db).Create(token).Error)
}

func (r *RefreshTokenRepository) FindByHashForUpdate(ctx context.Context, hash string) (*model.RefreshToken, error) {
	var token model.RefreshToken
	err := database.Conn(ctx, r.db).
		Clauses(lockForUpdate()).
		Where("token_hash = ?", hash).
		Take(&token).Error
	if err != nil {
		return nil, translate(err)
	}
	return &token, nil
}

func (r *RefreshTokenRepository) Revoke(ctx context.Context, id uuid.UUID, at time.Time) error {
	return database.Conn(ctx, r.db).
		Model(&model.RefreshToken{}).
		Where("id = ? AND revoked_at IS NULL", id).
		Update("revoked_at", at).Error
}

func (r *RefreshTokenRepository) RevokeByHash(ctx context.Context, userID uuid.UUID, hash string, at time.Time) error {
	return database.Conn(ctx, r.db).
		Model(&model.RefreshToken{}).
		Where("token_hash = ? AND user_id = ? AND revoked_at IS NULL", hash, userID).
		Update("revoked_at", at).Error
}

func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID, at time.Time) error {
	return database.Conn(ctx, r.db).
		Model(&model.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", at).Error
}
