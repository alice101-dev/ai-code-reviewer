package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/alice101-dev/ai-code-reviewer/internal/apperror"
	"github.com/alice101-dev/ai-code-reviewer/internal/httpapi/response"
)

const userIDKey = "auth.user_id"

var (
	errMissingToken = apperror.New(apperror.CodeUnauthenticated, "missing bearer token")
	errInvalidToken = apperror.New(apperror.CodeUnauthenticated, "invalid or expired access token")
)

type TokenParser interface {
	Parse(raw string) (uuid.UUID, error)
}

func Authenticate(tokens TokenParser) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			unauthorized(c, errMissingToken)
			return
		}

		userID, err := tokens.Parse(raw)
		if err != nil {
			unauthorized(c, errInvalidToken)
			return
		}

		c.Set(userIDKey, userID)
		c.Next()
	}
}

func UserID(c *gin.Context) (uuid.UUID, bool) {
	value, ok := c.Get(userIDKey)
	if !ok {
		return uuid.Nil, false
	}
	userID, ok := value.(uuid.UUID)
	return userID, ok
}

func bearerToken(header string) (string, bool) {
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, token != ""
}

func unauthorized(c *gin.Context, err *apperror.Error) {
	c.Header("WWW-Authenticate", `Bearer realm="api"`)
	response.Error(c, err)
}
