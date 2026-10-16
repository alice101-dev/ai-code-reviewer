package handler

import (
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/alice101-dev/ai-code-reviewer/internal/apperror"
	"github.com/alice101-dev/ai-code-reviewer/internal/httpapi/middleware"
	"github.com/alice101-dev/ai-code-reviewer/internal/httpapi/response"
)

const (
	idempotencyKeyHeader     = "Idempotency-Key"
	idempotentReplayedHeader = "Idempotent-Replayed"
)

var idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{8,64}$`)

var (
	errUnauthenticated       = apperror.New(apperror.CodeUnauthenticated, "authentication required")
	errInvalidWalletID       = apperror.New(apperror.CodeInvalidArgument, "wallet id must be a valid UUID")
	errInvalidIdempotencyKey = apperror.New(apperror.CodeInvalidArgument, "Idempotency-Key header must be 8-64 characters of letters, digits, '-' or '_'")
)

func currentUserID(c *gin.Context) (uuid.UUID, bool) {
	userID, ok := middleware.UserID(c)
	if !ok {
		response.Error(c, errUnauthenticated)
	}
	return userID, ok
}

func walletIDParam(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, errInvalidWalletID)
		return uuid.Nil, false
	}
	return id, true
}

func idempotencyKey(c *gin.Context) (string, bool) {
	key := c.GetHeader(idempotencyKeyHeader)
	if !idempotencyKeyPattern.MatchString(key) {
		response.Error(c, errInvalidIdempotencyKey)
		return "", false
	}
	return key, true
}
