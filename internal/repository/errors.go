package repository

import (
	"errors"

	"gorm.io/gorm"
)

var (
	ErrNotFound = errors.New("record not found")
	ErrConflict = errors.New("record conflicts with an existing record")
)

func translate(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return ErrNotFound
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return ErrConflict
	default:
		return err
	}
}
