package middleware

import (
	"context"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/alice101-dev/ai-code-reviewer/internal/apperror"
	"github.com/alice101-dev/ai-code-reviewer/internal/httpapi/response"
)

var errRateLimited = apperror.New(apperror.CodeRateLimited, "too many requests, please retry later")

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type IPRateLimiter struct {
	mu         sync.Mutex
	visitors   map[string]*visitor
	limit      rate.Limit
	burst      int
	ttl        time.Duration
	retryAfter string
	now        func() time.Time
}

func NewIPRateLimiter(rps float64, burst int, ttl time.Duration) *IPRateLimiter {
	return &IPRateLimiter{
		visitors:   make(map[string]*visitor),
		limit:      rate.Limit(rps),
		burst:      burst,
		ttl:        ttl,
		retryAfter: strconv.Itoa(int(math.Max(1, math.Ceil(1/rps)))),
		now:        time.Now,
	}
}

func (l *IPRateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	v, ok := l.visitors[key]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(l.limit, l.burst)}
		l.visitors[key] = v
	}
	v.lastSeen = l.now()

	return v.limiter.Allow()
}

func (l *IPRateLimiter) Run(ctx context.Context) {
	ticker := time.NewTicker(l.ttl)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.evictStale()
		}
	}
}

func (l *IPRateLimiter) evictStale() {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := l.now().Add(-l.ttl)
	for key, v := range l.visitors {
		if v.lastSeen.Before(cutoff) {
			delete(l.visitors, key)
		}
	}
}

func RateLimit(limiter *IPRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !limiter.Allow(c.ClientIP()) {
			c.Header("Retry-After", limiter.retryAfter)
			response.Error(c, errRateLimited)
			return
		}
		c.Next()
	}
}
