package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/alice101-dev/ai-code-reviewer/internal/config"
	"github.com/alice101-dev/ai-code-reviewer/internal/database"
	"github.com/alice101-dev/ai-code-reviewer/internal/httpapi"
	"github.com/alice101-dev/ai-code-reviewer/internal/httpapi/handler"
	"github.com/alice101-dev/ai-code-reviewer/internal/httpapi/middleware"
	"github.com/alice101-dev/ai-code-reviewer/internal/logger"
	"github.com/alice101-dev/ai-code-reviewer/internal/repository"
	"github.com/alice101-dev/ai-code-reviewer/internal/security"
	"github.com/alice101-dev/ai-code-reviewer/internal/service"
)

func Run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := logger.New(os.Stdout, cfg.Log, cfg.App)
	slog.SetDefault(log)

	db, err := database.Open(ctx, database.DSN(cfg.Database, cfg.App.Name), cfg.Database, log)
	if err != nil {
		return err
	}
	defer func() {
		if err := database.Close(db); err != nil {
			log.Error("close database", slog.Any("error", err))
		}
	}()

	txManager := database.NewTxManager(db)
	users := repository.NewUserRepository(db)
	refreshTokens := repository.NewRefreshTokenRepository(db)
	wallets := repository.NewWalletRepository(db)
	transactions := repository.NewTransactionRepository(db)

	tokens := security.NewTokenManager(cfg.Auth.JWTSecret, cfg.Auth.JWTIssuer, cfg.Auth.JWTAudience, cfg.Auth.AccessTokenTTL)
	hasher := security.NewPasswordHasher(cfg.Auth.BcryptCost)

	authService, err := service.NewAuthService(txManager, users, refreshTokens, hasher, tokens, cfg.Auth.RefreshTokenTTL)
	if err != nil {
		return err
	}
	walletService := service.NewWalletService(txManager, wallets, transactions, cfg.Wallet.SupportedCurrencies)
	paymentService := service.NewPaymentService(txManager, wallets, transactions, service.PaymentLimits{
		MaxTopUpAmount:    cfg.Wallet.MaxTopUpAmount,
		MaxTransferAmount: cfg.Wallet.MaxTransferAmount,
		MaxBalance:        cfg.Wallet.MaxBalance,
	})

	authLimiter := middleware.NewIPRateLimiter(cfg.Auth.RateLimitRPS, cfg.Auth.RateLimitBurst, cfg.Auth.RateLimitTTL)
	go authLimiter.Run(ctx)

	router, err := httpapi.NewRouter(httpapi.RouterDeps{
		Config:      cfg.HTTP,
		Logger:      log,
		Tokens:      tokens,
		AuthLimiter: authLimiter,
		Handlers: httpapi.Handlers{
			Health: handler.NewHealthHandler(database.Ping(db), cfg.Database.ConnectTimeout),
			Auth:   handler.NewAuthHandler(authService),
			Wallet: handler.NewWalletHandler(walletService, paymentService),
		},
	})
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              cfg.HTTP.Addr(),
		Handler:           router,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		BaseContext: func(net.Listener) context.Context {
			return context.WithoutCancel(ctx)
		},
	}

	return serve(ctx, server, cfg.HTTP.ShutdownTimeout, log)
}

func serve(ctx context.Context, server *http.Server, shutdownTimeout time.Duration, log *slog.Logger) error {
	errCh := make(chan error, 1)
	go func() {
		log.Info("http server listening", slog.String("addr", server.Addr))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	log.Info("shutting down http server", slog.Duration("timeout", shutdownTimeout))

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}

	log.Info("http server stopped")
	return nil
}
