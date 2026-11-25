package database

import (
	"context"

	"gorm.io/gorm"
)

type txKey struct{}

type TxManager struct {
	db *gorm.DB
}

func NewTxManager(db *gorm.DB) *TxManager {
	return &TxManager{db: db}
}

func (m *TxManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return fn(ctx)
	}

	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

func Conn(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return db.WithContext(ctx)
}
