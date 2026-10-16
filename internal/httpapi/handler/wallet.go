package handler

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/alice101-dev/ai-code-reviewer/internal/httpapi/response"
	"github.com/alice101-dev/ai-code-reviewer/internal/model"
	"github.com/alice101-dev/ai-code-reviewer/internal/service"
)

type WalletService interface {
	Create(ctx context.Context, userID uuid.UUID, currency string) (*model.Wallet, error)
	List(ctx context.Context, userID uuid.UUID) ([]model.Wallet, error)
	Get(ctx context.Context, userID, walletID uuid.UUID) (*model.Wallet, error)
	ListActivity(ctx context.Context, userID, walletID uuid.UUID, filter model.ActivityFilter, page service.Pagination) (*service.ActivityPage, error)
	Summary(ctx context.Context, userID, walletID uuid.UUID, filter model.ActivityFilter) (*model.WalletSummary, error)
}

type PaymentService interface {
	TopUp(ctx context.Context, in service.TopUpInput) (*service.PaymentResult, error)
	Transfer(ctx context.Context, in service.TransferInput) (*service.PaymentResult, error)
}

type WalletHandler struct {
	wallets  WalletService
	payments PaymentService
}

func NewWalletHandler(wallets WalletService, payments PaymentService) *WalletHandler {
	return &WalletHandler{wallets: wallets, payments: payments}
}

type createWalletRequest struct {
	Currency string `json:"currency" binding:"required,len=3,alpha"`
}

type topUpRequest struct {
	Amount int64 `json:"amount" binding:"required,gt=0"`
}

type transferRequest struct {
	SourceWalletID      uuid.UUID `json:"source_wallet_id" binding:"required"`
	DestinationWalletID uuid.UUID `json:"destination_wallet_id" binding:"required"`
	Amount              int64     `json:"amount" binding:"required,gt=0"`
	Description         string    `json:"description" binding:"max=255"`
}

type listActivityQuery struct {
	Page      int       `form:"page,default=1" binding:"min=1"`
	PageSize  int       `form:"page_size,default=20" binding:"min=1,max=100"`
	Type      string    `form:"type" binding:"omitempty,oneof=topup transfer"`
	Direction string    `form:"direction" binding:"omitempty,oneof=debit credit"`
	From      time.Time `form:"from"`
	To        time.Time `form:"to"`
}

func (q listActivityQuery) filter() model.ActivityFilter {
	return model.ActivityFilter{
		Type:      model.TransactionType(q.Type),
		Direction: model.EntryDirection(q.Direction),
		From:      q.From,
		To:        q.To,
	}
}

func (h *WalletHandler) Create(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	var req createWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}

	wallet, err := h.wallets.Create(c.Request.Context(), userID, req.Currency)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.Created(c, newWalletResponse(wallet))
}

func (h *WalletHandler) List(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	wallets, err := h.wallets.List(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, err)
		return
	}

	items := make([]walletResponse, 0, len(wallets))
	for i := range wallets {
		items = append(items, newWalletResponse(&wallets[i]))
	}

	response.OK(c, items)
}

func (h *WalletHandler) Get(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	walletID, ok := walletIDParam(c)
	if !ok {
		return
	}

	wallet, err := h.wallets.Get(c.Request.Context(), userID, walletID)
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, newWalletResponse(wallet))
}

func (h *WalletHandler) ListActivity(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	walletID, ok := walletIDParam(c)
	if !ok {
		return
	}

	var query listActivityQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.QueryError(c, err)
		return
	}

	page := service.Pagination{Page: query.Page, PageSize: query.PageSize}
	result, err := h.wallets.ListActivity(c.Request.Context(), userID, walletID, query.filter(), page)
	if err != nil {
		response.Error(c, err)
		return
	}

	items := make([]activityResponse, 0, len(result.Items))
	for _, a := range result.Items {
		items = append(items, newActivityResponse(a))
	}

	response.OKWithMeta(c, items, newPaginationMeta(page, result.TotalItems))
}

type summaryQuery struct {
	Type string    `form:"type" binding:"omitempty,oneof=topup transfer"`
	From time.Time `form:"from"`
	To   time.Time `form:"to"`
}

func (q summaryQuery) filter() model.ActivityFilter {
	return model.ActivityFilter{
		Type: model.TransactionType(q.Type),
		From: q.From,
		To:   q.To,
	}
}

func (h *WalletHandler) Summary(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	walletID, ok := walletIDParam(c)
	if !ok {
		return
	}

	var query summaryQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		response.QueryError(c, err)
		return
	}

	summary, err := h.wallets.Summary(c.Request.Context(), userID, walletID, query.filter())
	if err != nil {
		response.Error(c, err)
		return
	}

	response.OK(c, newSummaryResponse(summary))
}

func (h *WalletHandler) TopUp(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	walletID, ok := walletIDParam(c)
	if !ok {
		return
	}
	key, ok := idempotencyKey(c)
	if !ok {
		return
	}

	var req topUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}

	result, err := h.payments.TopUp(c.Request.Context(), service.TopUpInput{
		UserID:         userID,
		WalletID:       walletID,
		Amount:         req.Amount,
		IdempotencyKey: key,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	writePaymentResult(c, result)
}

func (h *WalletHandler) Transfer(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	key, ok := idempotencyKey(c)
	if !ok {
		return
	}

	var req transferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}

	result, err := h.payments.Transfer(c.Request.Context(), service.TransferInput{
		UserID:              userID,
		SourceWalletID:      req.SourceWalletID,
		DestinationWalletID: req.DestinationWalletID,
		Amount:              req.Amount,
		Description:         req.Description,
		IdempotencyKey:      key,
	})
	if err != nil {
		response.Error(c, err)
		return
	}

	writePaymentResult(c, result)
}

func writePaymentResult(c *gin.Context, result *service.PaymentResult) {
	if result.Replayed {
		c.Header(idempotentReplayedHeader, "true")
		response.OK(c, newTransactionResponse(result.Transaction))
		return
	}
	response.Created(c, newTransactionResponse(result.Transaction))
}
