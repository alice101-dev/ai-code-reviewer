package model

import (
	"time"

	"github.com/google/uuid"
)

type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusSuspended UserStatus = "suspended"
)

type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:uuidv7()"`
	Email        string
	PasswordHash string
	FullName     string
	Status       UserStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (User) TableName() string {
	return "users"
}

func (u *User) IsActive() bool {
	return u.Status == UserStatusActive
}
