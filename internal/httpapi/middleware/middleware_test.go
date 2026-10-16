package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/alice101-dev/ai-code-reviewer/internal/httpapi/response"
)

type stubTokenParser struct {
	userID uuid.UUID
	err    error
}

func (s stubTokenParser) Parse(string) (uuid.UUID, error) {
	return s.userID, s.err
}

func init() {
	gin.SetMode(gin.TestMode)
}

func TestAuthenticate(t *testing.T) {
	userID := uuid.New()

	tests := map[string]struct {
		header     string
		parser     stubTokenParser
		wantStatus int
	}{
		"missing header":   {header: "", parser: stubTokenParser{userID: userID}, wantStatus: http.StatusUnauthorized},
		"wrong scheme":     {header: "Basic abc", parser: stubTokenParser{userID: userID}, wantStatus: http.StatusUnauthorized},
		"empty bearer":     {header: "Bearer   ", parser: stubTokenParser{userID: userID}, wantStatus: http.StatusUnauthorized},
		"invalid token":    {header: "Bearer abc", parser: stubTokenParser{err: errors.New("bad")}, wantStatus: http.StatusUnauthorized},
		"valid token":      {header: "Bearer abc", parser: stubTokenParser{userID: userID}, wantStatus: http.StatusOK},
		"case insensitive": {header: "bearer abc", parser: stubTokenParser{userID: userID}, wantStatus: http.StatusOK},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := gin.New()
			r.GET("/", Authenticate(tt.parser), func(c *gin.Context) {
				id, ok := UserID(c)
				require.True(t, ok)
				assert.Equal(t, userID, id)
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantStatus == http.StatusUnauthorized {
				assert.NotEmpty(t, rec.Header().Get("WWW-Authenticate"))
			}
		})
	}
}

func TestRateLimit(t *testing.T) {
	limiter := NewIPRateLimiter(0.5, 2, time.Minute)

	r := gin.New()
	r.POST("/", RateLimit(limiter), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	send := func(remoteAddr string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
		req.RemoteAddr = remoteAddr
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	assert.Equal(t, http.StatusNoContent, send("10.0.0.1:1000").Code)
	assert.Equal(t, http.StatusNoContent, send("10.0.0.1:1001").Code)

	limited := send("10.0.0.1:1002")
	assert.Equal(t, http.StatusTooManyRequests, limited.Code)
	assert.Equal(t, "2", limited.Header().Get("Retry-After"))

	assert.Equal(t, http.StatusNoContent, send("10.0.0.2:1000").Code)
}

func TestIPRateLimiterEvictsStaleVisitors(t *testing.T) {
	now := time.Now()
	limiter := NewIPRateLimiter(1, 1, time.Minute)
	limiter.now = func() time.Time { return now }

	limiter.Allow("10.0.0.1")
	now = now.Add(2 * time.Minute)
	limiter.Allow("10.0.0.2")
	limiter.evictStale()

	assert.NotContains(t, limiter.visitors, "10.0.0.1")
	assert.Contains(t, limiter.visitors, "10.0.0.2")
}

func TestRequestID(t *testing.T) {
	r := gin.New()
	r.Use(RequestID())
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	tests := map[string]struct {
		incoming string
		keep     bool
	}{
		"propagates valid id":  {incoming: "req-123.abc_DEF", keep: true},
		"replaces invalid id":  {incoming: "bad id\nwith newline", keep: false},
		"generates missing id": {incoming: "", keep: false},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			if tt.incoming != "" {
				req.Header.Set(response.RequestIDHeader, tt.incoming)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			got := rec.Header().Get(response.RequestIDHeader)
			if tt.keep {
				assert.Equal(t, tt.incoming, got)
				return
			}
			_, err := uuid.Parse(got)
			assert.NoError(t, err)
		})
	}
}
