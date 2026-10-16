package handler

import (
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/alice101-dev/ai-code-reviewer/internal/model"
	"github.com/alice101-dev/ai-code-reviewer/internal/service"
)

type userResponse struct {
	ID        uuid.UUID        `json:"id"`
	Email     string           `json:"email"`
	FullName  string           `json:"full_name"`
	Status    model.UserStatus `json:"status"`
	CreatedAt time.Time        `json:"created_at"`
}

func newUserResponse(u *model.User) userResponse {
	return userResponse{
		ID:        u.ID,
		Email:     u.Email,
		FullName:  u.FullName,
		Status:    u.Status,
		CreatedAt: u.CreatedAt,
	}
}

type tokenResponse struct {
	TokenType             string    `json:"token_type"`
	AccessToken           string    `json:"access_token"`
	ExpiresIn             int64     `json:"expires_in"`
	AccessTokenExpiresAt  time.Time `json:"access_token_expires_at"`
	RefreshToken          string    `json:"refresh_token"`
	RefreshTokenExpiresAt time.Time `json:"refresh_token_expires_at"`
}

func newTokenResponse(p *service.TokenPair) tokenResponse {
	return tokenResponse{
		TokenType:             "Bearer",
		AccessToken:           p.AccessToken,
		ExpiresIn:             int64(math.Round(time.Until(p.AccessTokenExpiresAt).Seconds())),
		AccessTokenExpiresAt:  p.AccessTokenExpiresAt,
		RefreshToken:          p.RefreshToken,
		RefreshTokenExpiresAt: p.RefreshTokenExpiresAt,
	}
}

type walletResponse struct {
	ID        uuid.UUID          `json:"id"`
	Currency  string             `json:"currency"`
	Balance   int64              `json:"balance"`
	Status    model.WalletStatus `json:"status"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}

func newWalletResponse(w *model.Wallet) walletResponse {
	return walletResponse{
		ID:        w.ID,
		Currency:  w.Currency,
		Balance:   w.Balance,
		Status:    w.Status,
		CreatedAt: w.CreatedAt,
		UpdatedAt: w.UpdatedAt,
	}
}

type transactionResponse struct {
	ID                  uuid.UUID             `json:"id"`
	Type                model.TransactionType `json:"type"`
	SourceWalletID      *uuid.UUID            `json:"source_wallet_id"`
	DestinationWalletID uuid.UUID             `json:"destination_wallet_id"`
	Amount              int64                 `json:"amount"`
	Currency            string                `json:"currency"`
	Description         string                `json:"description"`
	CreatedAt           time.Time             `json:"created_at"`
}

func newTransactionResponse(t *model.Transaction) transactionResponse {
	return transactionResponse{
		ID:                  t.ID,
		Type:                t.Type,
		SourceWalletID:      t.SourceWalletID,
		DestinationWalletID: t.DestinationWalletID,
		Amount:              t.Amount,
		Currency:            t.Currency,
		Description:         t.Description,
		CreatedAt:           t.CreatedAt,
	}
}

type activityResponse struct {
	ID                   uuid.UUID             `json:"id"`
	TransactionID        uuid.UUID             `json:"transaction_id"`
	Type                 model.TransactionType `json:"type"`
	Direction            model.EntryDirection  `json:"direction"`
	Amount               int64                 `json:"amount"`
	BalanceAfter         int64                 `json:"balance_after"`
	Currency             string                `json:"currency"`
	Description          string                `json:"description"`
	CounterpartyWalletID *uuid.UUID            `json:"counterparty_wallet_id"`
	CreatedAt            time.Time             `json:"created_at"`
}

func newActivityResponse(a model.WalletActivity) activityResponse {
	return activityResponse{
		ID:                   a.EntryID,
		TransactionID:        a.TransactionID,
		Type:                 a.Type,
		Direction:            a.Direction,
		Amount:               a.Amount,
		BalanceAfter:         a.BalanceAfter,
		Currency:             a.Currency,
		Description:          a.Description,
		CounterpartyWalletID: a.CounterpartyWalletID,
		CreatedAt:            a.CreatedAt,
	}
}

type summaryResponse struct {
	Currency         string `json:"currency"`
	TotalCredits     int64  `json:"total_credits"`
	TotalDebits      int64  `json:"total_debits"`
	NetChange        int64  `json:"net_change"`
	TransactionCount int64  `json:"transaction_count"`
}

func newSummaryResponse(s *model.WalletSummary) summaryResponse {
	return summaryResponse{
		Currency:         s.Currency,
		TotalCredits:     s.TotalCredits,
		TotalDebits:      s.TotalDebits,
		NetChange:        s.NetChange,
		TransactionCount: s.TransactionCount,
	}
}

type paginationMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	TotalItems int64 `json:"total_items"`
	TotalPages int64 `json:"total_pages"`
}

func newPaginationMeta(p service.Pagination, total int64) map[string]paginationMeta {
	pageSize := int64(p.PageSize)
	return map[string]paginationMeta{
		"pagination": {
			Page:       p.Page,
			PageSize:   p.PageSize,
			TotalItems: total,
			TotalPages: (total + pageSize - 1) / pageSize,
		},
	}
}

type cursorMeta struct {
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

func newCursorMeta(next string) map[string]cursorMeta {
	return map[string]cursorMeta{
		"cursor": {
			NextCursor: next,
			HasMore:    next != "",
		},
	}
}
