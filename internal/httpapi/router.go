package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"github.com/alice101-dev/ai-code-reviewer/internal/apperror"
	"github.com/alice101-dev/ai-code-reviewer/internal/config"
	"github.com/alice101-dev/ai-code-reviewer/internal/httpapi/handler"
	"github.com/alice101-dev/ai-code-reviewer/internal/httpapi/middleware"
	"github.com/alice101-dev/ai-code-reviewer/internal/httpapi/response"
)

const (
	livenessPath  = "/healthz"
	readinessPath = "/readyz"
)

var registerValidatorOnce sync.Once

type Handlers struct {
	Health *handler.HealthHandler
	Auth   *handler.AuthHandler
	Wallet *handler.WalletHandler
}

type RouterDeps struct {
	Config      config.HTTP
	Logger      *slog.Logger
	Tokens      middleware.TokenParser
	AuthLimiter *middleware.IPRateLimiter
	Handlers    Handlers
}

func NewRouter(deps RouterDeps) (*gin.Engine, error) {
	gin.SetMode(deps.Config.GinMode)
	registerValidatorOnce.Do(useJSONFieldNames)

	r := gin.New()
	r.HandleMethodNotAllowed = true

	if err := r.SetTrustedProxies(deps.Config.TrustedProxies); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}

	r.Use(
		middleware.RequestID(),
		middleware.Logger(deps.Logger, livenessPath, readinessPath),
		middleware.Recovery(deps.Logger),
		middleware.SecurityHeaders(),
		middleware.BodyLimit(deps.Config.MaxBodyBytes),
	)

	if len(deps.Config.CORSAllowedOrigins) > 0 {
		r.Use(cors.New(cors.Config{
			AllowOrigins:     deps.Config.CORSAllowedOrigins,
			AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodOptions},
			AllowHeaders:     []string{"Authorization", "Content-Type", "Idempotency-Key", response.RequestIDHeader},
			ExposeHeaders:    []string{response.RequestIDHeader, "Idempotent-Replayed", "Retry-After"},
			AllowCredentials: false,
			MaxAge:           12 * time.Hour,
		}))
	}

	r.NoRoute(func(c *gin.Context) {
		response.Error(c, apperror.New(apperror.CodeNotFound, "route not found"))
	})
	r.NoMethod(func(c *gin.Context) {
		response.Error(c, apperror.New(apperror.CodeMethodNotAllowed, "method not allowed"))
	})

	h := deps.Handlers
	authenticate := middleware.Authenticate(deps.Tokens)

	r.GET(livenessPath, h.Health.Live)
	r.GET(readinessPath, h.Health.Ready)

	v1 := r.Group("/api/v1")

	auth := v1.Group("/auth")
	{
		limited := auth.Group("", middleware.RateLimit(deps.AuthLimiter))
		limited.POST("/register", h.Auth.Register)
		limited.POST("/login", h.Auth.Login)
		limited.POST("/refresh", h.Auth.Refresh)
		auth.POST("/logout", authenticate, h.Auth.Logout)
	}

	protected := v1.Group("", authenticate)
	{
		protected.GET("/users/me", h.Auth.Me)

		protected.POST("/wallets", h.Wallet.Create)
		protected.GET("/wallets", h.Wallet.List)
		protected.GET("/wallets/:id", h.Wallet.Get)
		protected.GET("/wallets/:id/transactions", h.Wallet.ListActivity)
		protected.POST("/wallets/:id/topups", h.Wallet.TopUp)

		protected.POST("/transfers", h.Wallet.Transfer)
	}

	return r, nil
}

func useJSONFieldNames() {
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return
	}

	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		for _, tag := range []string{"json", "form"} {
			name, _, _ := strings.Cut(field.Tag.Get(tag), ",")
			if name == "-" {
				return ""
			}
			if name != "" {
				return name
			}
		}
		return field.Name
	})
}
