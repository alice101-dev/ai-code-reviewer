package model

import (
	"time"

	"github.com/google/uuid"
)

type WalletStatus string

const (
	WalletStatusActive WalletStatus = "active"
	WalletStatusFrozen WalletStatus = "frozen"
	WalletStatusClosed WalletStatus = "closed"
)

type Wallet struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:uuidv7()"`
	UserID    uuid.UUID `gorm:"type:uuid"`
	Currency  string
	Balance   int64
	Status    WalletStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (Wallet) TableName() string {
	return "wallets"
}

func (w *Wallet) IsActive() bool {
	return w.Status == WalletStatusActive
}

type TransactionType string

const (
	TransactionTypeTopUp    TransactionType = "topup"
	TransactionTypeTransfer TransactionType = "transfer"
)

type Transaction struct {
	ID                  uuid.UUID `gorm:"type:uuid;primaryKey;default:uuidv7()"`
	UserID              uuid.UUID `gorm:"type:uuid"`
	Type                TransactionType
	SourceWalletID      *uuid.UUID `gorm:"type:uuid"`
	DestinationWalletID uuid.UUID  `gorm:"type:uuid"`
	Amount              int64
	Currency            string
	Description         string
	IdempotencyKey      string
	CreatedAt           time.Time
}

func (Transaction) TableName() string {
	return "transactions"
}

type EntryDirection string

const (
	EntryDirectionDebit  EntryDirection = "debit"
	EntryDirectionCredit EntryDirection = "credit"
)

type LedgerEntry struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:uuidv7()"`
	TransactionID uuid.UUID `gorm:"type:uuid"`
	WalletID      uuid.UUID `gorm:"type:uuid"`
	Direction     EntryDirection
	Amount        int64
	BalanceAfter  int64
	CreatedAt     time.Time
}

func (LedgerEntry) TableName() string {
	return "ledger_entries"
}

type ActivityFilter struct {
	Type      TransactionType
	Direction EntryDirection
	From      time.Time
	To        time.Time
}

type WalletActivity struct {
	EntryID              uuid.UUID
	TransactionID        uuid.UUID
	Type                 TransactionType
	Direction            EntryDirection
	Amount               int64
	BalanceAfter         int64
	Currency             string
	Description          string
	CounterpartyWalletID *uuid.UUID
	CreatedAt            time.Time
}
