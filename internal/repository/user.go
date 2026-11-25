package repository

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/alice101-dev/ai-code-reviewer/internal/database"
	"github.com/alice101-dev/ai-code-reviewer/internal/model"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	return translate(database.Conn(ctx, r.db).Create(user).Error)
}

func (r *UserRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	var user model.User
	if err := database.Conn(ctx, r.db).Where("id = ?", id).Take(&user).Error; err != nil {
		return nil, translate(err)
	}
	return &user, nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	if err := database.Conn(ctx, r.db).Where("email = ?", email).Take(&user).Error; err != nil {
		return nil, translate(err)
	}
	return &user, nil
}
