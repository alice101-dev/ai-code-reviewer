package database

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/alice101-dev/ai-code-reviewer/internal/config"
)

func DSN(cfg config.Database, applicationName string) string {
	query := url.Values{}
	query.Set("sslmode", cfg.SSLMode)
	query.Set("application_name", applicationName)
	query.Set("connect_timeout", strconv.Itoa(int(cfg.ConnectTimeout.Seconds())))

	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.User, cfg.Password),
		Host:     net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Path:     cfg.Name,
		RawQuery: query.Encode(),
	}
	return u.String()
}

func Open(ctx context.Context, dsn string, cfg config.Database, log *slog.Logger) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.NewSlogLogger(log.With(slog.String("component", "gorm")), logger.Config{
			SlowThreshold:             cfg.SlowQueryThreshold,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
		}),
		TranslateError:         true,
		SkipDefaultTransaction: true,
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	if err := sqlDB.PingContext(pingCtx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return db, nil
}

func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func Ping(db *gorm.DB) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.PingContext(ctx)
	}
}
