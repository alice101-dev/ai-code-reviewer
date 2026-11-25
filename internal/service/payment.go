package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/alice101-dev/ai-code-reviewer/internal/model"
	"github.com/alice101-dev/ai-code-reviewer/internal/repository"
)

type PaymentLimits struct {
	MaxTopUpAmount    int64
	MaxTransferAmount int64
	MaxBalance        int64
}

type TopUpInput struct {
	UserID         uuid.UUID
	WalletID       uuid.UUID
	Amount         int64
	IdempotencyKey string
}

type TransferInput struct {
	UserID              uuid.UUID
	SourceWalletID      uuid.UUID
	DestinationWalletID uuid.UUID
	Amount              int64
	Description         string
	IdempotencyKey      string
}

type PaymentResult struct {
	Transaction *model.Transaction
	Replayed    bool
}

type PaymentService struct {
	tx           TxRunner
	wallets      WalletStore
	transactions TransactionStore
	limits       PaymentLimits
}

func NewPaymentService(tx TxRunner, wallets WalletStore, transactions TransactionStore, limits PaymentLimits) *PaymentService {
	return &PaymentService{
		tx:           tx,
		wallets:      wallets,
		transactions: transactions,
		limits:       limits,
	}
}

func (s *PaymentService) TopUp(ctx context.Context, in TopUpInput) (*PaymentResult, error) {
	if in.Amount <= 0 || in.Amount > s.limits.MaxTopUpAmount {
		return nil, ErrAmountOutOfRange
	}

	matches := func(t *model.Transaction) bool {
		return t.Type == model.TransactionTypeTopUp &&
			t.DestinationWalletID == in.WalletID &&
			t.Amount == in.Amount
	}

	return s.execute(ctx, in.UserID, in.IdempotencyKey, matches, func(ctx context.Context) (*model.Transaction, error) {
		wallets, err := s.wallets.LockByIDs(ctx, in.WalletID)
		if err != nil {
			return nil, fmt.Errorf("lock wallet: %w", err)
		}
		if len(wallets) != 1 || wallets[0].UserID != in.UserID {
			return nil, ErrWalletNotFound
		}

		wallet := wallets[0]
		if !wallet.IsActive() {
			return nil, ErrWalletNotActive
		}
		if in.Amount > s.limits.MaxBalance-wallet.Balance {
			return nil, ErrBalanceLimitExceeded
		}

		txn := &model.Transaction{
			UserID:              in.UserID,
			Type:                model.TransactionTypeTopUp,
			DestinationWalletID: wallet.ID,
			Amount:              in.Amount,
			Currency:            wallet.Currency,
			IdempotencyKey:      in.IdempotencyKey,
		}
		if err := s.transactions.Create(ctx, txn); err != nil {
			return nil, fmt.Errorf("create transaction: %w", err)
		}

		balance := wallet.Balance + in.Amount
		if err := s.wallets.UpdateBalance(ctx, wallet.ID, balance); err != nil {
			return nil, fmt.Errorf("update wallet balance: %w", err)
		}

		entries := []model.LedgerEntry{{
			TransactionID: txn.ID,
			WalletID:      wallet.ID,
			Direction:     model.EntryDirectionCredit,
			Amount:        in.Amount,
			BalanceAfter:  balance,
		}}
		if err := s.transactions.CreateEntries(ctx, entries); err != nil {
			return nil, fmt.Errorf("create ledger entries: %w", err)
		}

		return txn, nil
	})
}

func (s *PaymentService) Transfer(ctx context.Context, in TransferInput) (*PaymentResult, error) {
	if in.Amount <= 0 || in.Amount > s.limits.MaxTransferAmount {
		return nil, ErrAmountOutOfRange
	}
	if in.SourceWalletID == in.DestinationWalletID {
		return nil, ErrSameWallet
	}

	description := strings.TrimSpace(in.Description)

	matches := func(t *model.Transaction) bool {
		return t.Type == model.TransactionTypeTransfer &&
			t.SourceWalletID != nil &&
			*t.SourceWalletID == in.SourceWalletID &&
			t.DestinationWalletID == in.DestinationWalletID &&
			t.Amount == in.Amount
	}

	return s.execute(ctx, in.UserID, in.IdempotencyKey, matches, func(ctx context.Context) (*model.Transaction, error) {
		wallets, err := s.wallets.LockByIDs(ctx, in.SourceWalletID, in.DestinationWalletID)
		if err != nil {
			return nil, fmt.Errorf("lock wallets: %w", err)
		}

		var source, destination *model.Wallet
		for i := range wallets {
			switch wallets[i].ID {
			case in.SourceWalletID:
				source = &wallets[i]
			case in.DestinationWalletID:
				destination = &wallets[i]
			}
		}

		if source == nil || source.UserID != in.UserID {
			return nil, ErrWalletNotFound
		}
		if destination == nil {
			return nil, ErrDestinationWalletNotFound
		}
		if !source.IsActive() || !destination.IsActive() {
			return nil, ErrWalletNotActive
		}
		if source.Currency != destination.Currency {
			return nil, ErrCurrencyMismatch
		}
		if source.Balance < in.Amount {
			return nil, ErrInsufficientFunds
		}
		if in.Amount > s.limits.MaxBalance-destination.Balance {
			return nil, ErrBalanceLimitExceeded
		}

		sourceID := source.ID
		txn := &model.Transaction{
			UserID:              in.UserID,
			Type:                model.TransactionTypeTransfer,
			SourceWalletID:      &sourceID,
			DestinationWalletID: destination.ID,
			Amount:              in.Amount,
			Currency:            source.Currency,
			Description:         description,
			IdempotencyKey:      in.IdempotencyKey,
		}
		if err := s.transactions.Create(ctx, txn); err != nil {
			return nil, fmt.Errorf("create transaction: %w", err)
		}

		sourceBalance := source.Balance - in.Amount
		destinationBalance := destination.Balance + in.Amount

		if err := s.wallets.UpdateBalance(ctx, source.ID, sourceBalance); err != nil {
			return nil, fmt.Errorf("debit source wallet: %w", err)
		}
		if err := s.wallets.UpdateBalance(ctx, destination.ID, destinationBalance); err != nil {
			return nil, fmt.Errorf("credit destination wallet: %w", err)
		}

		entries := []model.LedgerEntry{
			{
				TransactionID: txn.ID,
				WalletID:      source.ID,
				Direction:     model.EntryDirectionDebit,
				Amount:        in.Amount,
				BalanceAfter:  sourceBalance,
			},
			{
				TransactionID: txn.ID,
				WalletID:      destination.ID,
				Direction:     model.EntryDirectionCredit,
				Amount:        in.Amount,
				BalanceAfter:  destinationBalance,
			},
		}
		if err := s.transactions.CreateEntries(ctx, entries); err != nil {
			return nil, fmt.Errorf("create ledger entries: %w", err)
		}

		return txn, nil
	})
}

func (s *PaymentService) execute(
	ctx context.Context,
	userID uuid.UUID,
	idempotencyKey string,
	matches func(*model.Transaction) bool,
	apply func(ctx context.Context) (*model.Transaction, error),
) (*PaymentResult, error) {
	var result *PaymentResult

	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		existing, err := s.transactions.FindByIdempotencyKey(ctx, userID, idempotencyKey)
		switch {
		case err == nil:
			if !matches(existing) {
				return ErrIdempotencyKeyReused
			}
			result = &PaymentResult{Transaction: existing, Replayed: true}
			return nil
		case !errors.Is(err, repository.ErrNotFound):
			return fmt.Errorf("find transaction by idempotency key: %w", err)
		}

		txn, err := apply(ctx)
		if err != nil {
			return err
		}
		result = &PaymentResult{Transaction: txn}
		return nil
	})

	if errors.Is(err, repository.ErrConflict) {
		return s.replay(ctx, userID, idempotencyKey, matches)
	}
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *PaymentService) replay(ctx context.Context, userID uuid.UUID, idempotencyKey string, matches func(*model.Transaction) bool) (*PaymentResult, error) {
	existing, err := s.transactions.FindByIdempotencyKey(ctx, userID, idempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("load concurrent idempotent transaction: %w", err)
	}
	if !matches(existing) {
		return nil, ErrIdempotencyKeyReused
	}
	return &PaymentResult{Transaction: existing, Replayed: true}, nil
}
