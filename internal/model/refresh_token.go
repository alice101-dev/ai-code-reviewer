package model

import (
	"time"

	"github.com/google/uuid"
)

type RefreshToken struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:uuidv7()"`
	UserID    uuid.UUID `gorm:"type:uuid"`
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

func (RefreshToken) TableName() string {
	return "refresh_tokens"
}

func (t *RefreshToken) IsRevoked() bool {
	return t.RevokedAt != nil
}

func (t *RefreshToken) IsExpired(now time.Time) bool {
	return !now.Before(t.ExpiresAt)
}
