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

type Pagination struct {
	Page     int
	PageSize int
}

func (p Pagination) Offset() int {
	return (p.Page - 1) * p.PageSize
}

type ActivityPage struct {
	Items      []model.WalletActivity
	TotalItems int64
}

type CursorPage struct {
	Cursor string
	Limit  int
}

type ActivityFeed struct {
	Items      []model.WalletActivity
	NextCursor string
}

type WalletService struct {
	wallets      WalletStore
	transactions TransactionStore
	currencies   map[string]struct{}
}

func NewWalletService(wallets WalletStore, transactions TransactionStore, supportedCurrencies []string) *WalletService {
	currencies := make(map[string]struct{}, len(supportedCurrencies))
	for _, c := range supportedCurrencies {
		currencies[strings.ToUpper(c)] = struct{}{}
	}

	return &WalletService{
		wallets:      wallets,
		transactions: transactions,
		currencies:   currencies,
	}
}

func (s *WalletService) Create(ctx context.Context, userID uuid.UUID, currency string) (*model.Wallet, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if _, ok := s.currencies[currency]; !ok {
		return nil, ErrUnsupportedCurrency
	}

	wallet := &model.Wallet{
		UserID:   userID,
		Currency: currency,
		Status:   model.WalletStatusActive,
	}

	if err := s.wallets.Create(ctx, wallet); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, ErrWalletAlreadyExists
		}
		return nil, fmt.Errorf("create wallet: %w", err)
	}

	return wallet, nil
}

func (s *WalletService) List(ctx context.Context, userID uuid.UUID) ([]model.Wallet, error) {
	wallets, err := s.wallets.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list wallets: %w", err)
	}
	return wallets, nil
}

func (s *WalletService) Get(ctx context.Context, userID, walletID uuid.UUID) (*model.Wallet, error) {
	wallet, err := s.wallets.FindByIDForUser(ctx, walletID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrWalletNotFound
		}
		return nil, fmt.Errorf("find wallet: %w", err)
	}
	return wallet, nil
}

func (s *WalletService) ListActivity(ctx context.Context, userID, walletID uuid.UUID, filter model.ActivityFilter, page Pagination) (*ActivityPage, error) {
	if !filter.From.IsZero() && !filter.To.IsZero() && !filter.From.Before(filter.To) {
		return nil, ErrInvalidTimeRange
	}

	if _, err := s.Get(ctx, userID, walletID); err != nil {
		return nil, err
	}

	total, err := s.transactions.CountWalletActivity(ctx, walletID, filter)
	if err != nil {
		return nil, fmt.Errorf("count wallet activity: %w", err)
	}

	items, err := s.transactions.ListWalletActivity(ctx, walletID, filter, page.PageSize, page.Offset())
	if err != nil {
		return nil, fmt.Errorf("list wallet activity: %w", err)
	}

	return &ActivityPage{Items: items, TotalItems: total}, nil
}

func (s *WalletService) ListActivityFeed(ctx context.Context, userID, walletID uuid.UUID, filter model.ActivityFilter, page CursorPage) (*ActivityFeed, error) {
	if !filter.From.IsZero() && !filter.To.IsZero() && !filter.From.Before(filter.To) {
		return nil, ErrInvalidTimeRange
	}

	var before *model.ActivityCursor
	if page.Cursor != "" {
		cursor, err := decodeActivityCursor(page.Cursor)
		if err != nil {
			return nil, ErrInvalidCursor
		}
		before = &cursor
	}

	if _, err := s.Get(ctx, userID, walletID); err != nil {
		return nil, err
	}

	items, err := s.transactions.ListWalletActivityBefore(ctx, walletID, filter, before, page.Limit+1)
	if err != nil {
		return nil, fmt.Errorf("list wallet activity feed: %w", err)
	}

	feed := &ActivityFeed{Items: items}
	if len(items) > page.Limit {
		feed.Items = items[:page.Limit]
		last := feed.Items[len(feed.Items)-1]
		feed.NextCursor = encodeActivityCursor(model.ActivityCursor{CreatedAt: last.CreatedAt, EntryID: last.EntryID})
	}

	return feed, nil
}

func (s *WalletService) Summary(ctx context.Context, userID, walletID uuid.UUID, filter model.ActivityFilter) (*model.WalletSummary, error) {
	if !filter.From.IsZero() && !filter.To.IsZero() && !filter.From.Before(filter.To) {
		return nil, ErrInvalidTimeRange
	}

	wallet, err := s.Get(ctx, userID, walletID)
	if err != nil {
		return nil, err
	}

	summary, err := s.transactions.SummarizeWalletActivity(ctx, walletID, filter)
	if err != nil {
		return nil, fmt.Errorf("summarize wallet activity: %w", err)
	}

	summary.Currency = wallet.Currency
	summary.NetChange = summary.TotalCredits - summary.TotalDebits

	return &summary, nil
}
