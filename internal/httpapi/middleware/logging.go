package middleware

import (
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/alice101-dev/ai-code-reviewer/internal/apperror"
	"github.com/alice101-dev/ai-code-reviewer/internal/httpapi/response"
)

func Logger(log *slog.Logger, skipPaths ...string) gin.HandlerFunc {
	skip := make(map[string]struct{}, len(skipPaths))
	for _, p := range skipPaths {
		skip[p] = struct{}{}
	}

	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		status := c.Writer.Status()
		if _, ok := skip[c.Request.URL.Path]; ok && status < http.StatusInternalServerError {
			return
		}

		attrs := []slog.Attr{
			slog.String("request_id", c.Writer.Header().Get(response.RequestIDHeader)),
			slog.String("method", c.Request.Method),
			slog.String("route", c.FullPath()),
			slog.String("path", c.Request.URL.Path),
			slog.Int("status", status),
			slog.Int("bytes", c.Writer.Size()),
			slog.Duration("latency", time.Since(start)),
			slog.String("client_ip", c.ClientIP()),
			slog.String("user_agent", c.Request.UserAgent()),
		}
		if userID, ok := UserID(c); ok {
			attrs = append(attrs, slog.String("user_id", userID.String()))
		}
		if errs := c.Errors.ByType(gin.ErrorTypePrivate); len(errs) > 0 {
			attrs = append(attrs, slog.String("error", errs.String()))
		}

		level := slog.LevelInfo
		switch {
		case status >= http.StatusInternalServerError:
			level = slog.LevelError
		case status >= http.StatusBadRequest:
			level = slog.LevelWarn
		}

		log.LogAttrs(c.Request.Context(), level, "http request", attrs...)
	}
}

func Recovery(log *slog.Logger) gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, recovered any) {
		log.ErrorContext(c.Request.Context(), "panic recovered",
			slog.String("request_id", c.Writer.Header().Get(response.RequestIDHeader)),
			slog.Any("panic", recovered),
			slog.String("stack", string(debug.Stack())),
		)
		response.Error(c, apperror.New(apperror.CodeInternal, "internal server error"))
	})
}
