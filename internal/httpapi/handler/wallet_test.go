package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alice101-dev/ai-code-reviewer/internal/config"
	"github.com/alice101-dev/ai-code-reviewer/internal/httpapi"
	"github.com/alice101-dev/ai-code-reviewer/internal/httpapi/handler"
	"github.com/alice101-dev/ai-code-reviewer/internal/httpapi/middleware"
	"github.com/alice101-dev/ai-code-reviewer/internal/model"
	"github.com/alice101-dev/ai-code-reviewer/internal/service"
)

var errNotImplemented = errors.New("not implemented")

type stubTokens struct {
	userID uuid.UUID
}

func (s stubTokens) Parse(string) (uuid.UUID, error) {
	return s.userID, nil
}

type fakeWalletService struct {
	calls     int
	gotUserID uuid.UUID
	gotWallet uuid.UUID
	gotFilter model.ActivityFilter
	gotPage   service.Pagination
	result    *service.ActivityPage
	err       error
}

func (f *fakeWalletService) Create(context.Context, uuid.UUID, string) (*model.Wallet, error) {
	return nil, errNotImplemented
}

func (f *fakeWalletService) List(context.Context, uuid.UUID) ([]model.Wallet, error) {
	return nil, errNotImplemented
}

func (f *fakeWalletService) Get(context.Context, uuid.UUID, uuid.UUID) (*model.Wallet, error) {
	return nil, errNotImplemented
}

func (f *fakeWalletService) ListActivity(_ context.Context, userID, walletID uuid.UUID, filter model.ActivityFilter, page service.Pagination) (*service.ActivityPage, error) {
	f.calls++
	f.gotUserID = userID
	f.gotWallet = walletID
	f.gotFilter = filter
	f.gotPage = page
	if f.err != nil {
		return nil, f.err
	}
	if f.result != nil {
		return f.result, nil
	}
	return &service.ActivityPage{}, nil
}

func (f *fakeWalletService) Summary(_ context.Context, userID, walletID uuid.UUID, filter model.ActivityFilter) (*model.WalletSummary, error) {
	f.calls++
	f.gotUserID = userID
	f.gotWallet = walletID
	f.gotFilter = filter
	if f.err != nil {
		return nil, f.err
	}
	return &model.WalletSummary{Currency: "IDR"}, nil
}

type fakePaymentService struct{}

func (fakePaymentService) TopUp(context.Context, service.TopUpInput) (*service.PaymentResult, error) {
	return nil, errNotImplemented
}

func (fakePaymentService) Transfer(context.Context, service.TransferInput) (*service.PaymentResult, error) {
	return nil, errNotImplemented
}

type apiResponse struct {
	Data []map[string]any `json:"data"`
	Meta struct {
		Pagination struct {
			Page       int   `json:"page"`
			PageSize   int   `json:"page_size"`
			TotalItems int64 `json:"total_items"`
			TotalPages int64 `json:"total_pages"`
		} `json:"pagination"`
	} `json:"meta"`
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details []struct {
			Field string `json:"field"`
			Rule  string `json:"rule"`
		} `json:"details"`
	} `json:"error"`
}

func newTestRouter(t *testing.T, userID uuid.UUID, wallets handler.WalletService) *gin.Engine {
	t.Helper()

	router, err := httpapi.NewRouter(httpapi.RouterDeps{
		Config: config.HTTP{
			GinMode:      gin.TestMode,
			MaxBodyBytes: 1 << 20,
		},
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		Tokens:      stubTokens{userID: userID},
		AuthLimiter: middleware.NewIPRateLimiter(100, 100, time.Minute),
		Handlers: httpapi.Handlers{
			Health: handler.NewHealthHandler(func(context.Context) error { return nil }, time.Second),
			Auth:   handler.NewAuthHandler(nil),
			Wallet: handler.NewWalletHandler(wallets, fakePaymentService{}),
		},
	})
	require.NoError(t, err)
	return router
}

func getActivity(t *testing.T, router *gin.Engine, walletID string, query url.Values) (int, apiResponse) {
	t.Helper()

	target := "/api/v1/wallets/" + walletID + "/transactions"
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var body apiResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return rec.Code, body
}

func TestWalletHandler_ListActivityOmittedFiltersUseDefaults(t *testing.T) {
	userID, walletID := uuid.New(), uuid.New()
	wallets := &fakeWalletService{}
	router := newTestRouter(t, userID, wallets)

	status, body := getActivity(t, router, walletID.String(), nil)

	require.Equal(t, http.StatusOK, status)
	require.Equal(t, 1, wallets.calls)
	assert.Equal(t, userID, wallets.gotUserID)
	assert.Equal(t, walletID, wallets.gotWallet)
	assert.Equal(t, model.ActivityFilter{}, wallets.gotFilter)
	assert.Equal(t, service.Pagination{Page: 1, PageSize: 20}, wallets.gotPage)
	assert.Empty(t, body.Data)
}

func TestWalletHandler_ListActivityParsesFilters(t *testing.T) {
	wallets := &fakeWalletService{
		result: &service.ActivityPage{
			Items:      []model.WalletActivity{{EntryID: uuid.New(), Type: model.TransactionTypeTransfer, Direction: model.EntryDirectionDebit, Amount: 500}},
			TotalItems: 7,
		},
	}
	router := newTestRouter(t, uuid.New(), wallets)

	status, body := getActivity(t, router, uuid.NewString(), url.Values{
		"type":      {"transfer"},
		"direction": {"debit"},
		"from":      {"2026-10-01T07:00:00+07:00"},
		"to":        {"2026-11-01T00:00:00Z"},
		"page":      {"2"},
		"page_size": {"5"},
	})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, model.TransactionTypeTransfer, wallets.gotFilter.Type)
	assert.Equal(t, model.EntryDirectionDebit, wallets.gotFilter.Direction)
	assert.True(t, wallets.gotFilter.From.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)))
	assert.True(t, wallets.gotFilter.To.Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)))
	assert.Equal(t, service.Pagination{Page: 2, PageSize: 5}, wallets.gotPage)

	require.Len(t, body.Data, 1)
	assert.Equal(t, "transfer", body.Data[0]["type"])
	assert.Equal(t, 2, body.Meta.Pagination.Page)
	assert.Equal(t, int64(7), body.Meta.Pagination.TotalItems)
	assert.Equal(t, int64(2), body.Meta.Pagination.TotalPages)
}

func TestWalletHandler_ListActivityRejectsInvalidQuery(t *testing.T) {
	tests := map[string]struct {
		query       url.Values
		wantMessage string
		wantField   string
		wantRule    string
	}{
		"unknown type":        {query: url.Values{"type": {"refund"}}, wantMessage: "request validation failed", wantField: "type", wantRule: "oneof"},
		"unknown direction":   {query: url.Values{"direction": {"in"}}, wantMessage: "request validation failed", wantField: "direction", wantRule: "oneof"},
		"page size above max": {query: url.Values{"page_size": {"101"}}, wantMessage: "request validation failed", wantField: "page_size", wantRule: "max"},
		"page below min":      {query: url.Values{"page": {"0"}}, wantMessage: "request validation failed", wantField: "page", wantRule: "min"},
		"malformed from":      {query: url.Values{"from": {"yesterday"}}, wantMessage: "query parameters are malformed"},
		"date without time":   {query: url.Values{"to": {"2026-10-01"}}, wantMessage: "query parameters are malformed"},
		"non numeric page":    {query: url.Values{"page": {"two"}}, wantMessage: "query parameters are malformed"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			wallets := &fakeWalletService{}
			router := newTestRouter(t, uuid.New(), wallets)

			status, body := getActivity(t, router, uuid.NewString(), tt.query)

			assert.Equal(t, http.StatusBadRequest, status)
			assert.Equal(t, "invalid_argument", body.Error.Code)
			assert.Equal(t, tt.wantMessage, body.Error.Message)
			assert.Zero(t, wallets.calls)

			if tt.wantField != "" {
				require.Len(t, body.Error.Details, 1)
				assert.Equal(t, tt.wantField, body.Error.Details[0].Field)
				assert.Equal(t, tt.wantRule, body.Error.Details[0].Rule)
			}
		})
	}
}

func TestWalletHandler_ListActivityRejectsInvalidWalletID(t *testing.T) {
	wallets := &fakeWalletService{}
	router := newTestRouter(t, uuid.New(), wallets)

	status, body := getActivity(t, router, "not-a-uuid", nil)

	assert.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "wallet id must be a valid UUID", body.Error.Message)
	assert.Zero(t, wallets.calls)
}

func TestWalletHandler_ListActivityMapsServiceErrors(t *testing.T) {
	tests := map[string]struct {
		err        error
		wantStatus int
		wantCode   string
	}{
		"invalid time range": {err: service.ErrInvalidTimeRange, wantStatus: http.StatusBadRequest, wantCode: "invalid_argument"},
		"wallet not found":   {err: service.ErrWalletNotFound, wantStatus: http.StatusNotFound, wantCode: "not_found"},
		"unexpected error":   {err: errors.New("connection reset"), wantStatus: http.StatusInternalServerError, wantCode: "internal"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			router := newTestRouter(t, uuid.New(), &fakeWalletService{err: tt.err})

			status, body := getActivity(t, router, uuid.NewString(), nil)

			assert.Equal(t, tt.wantStatus, status)
			assert.Equal(t, tt.wantCode, body.Error.Code)
			assert.NotContains(t, body.Error.Message, "connection reset")
		})
	}
}
