package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

const minJWTSecretLength = 32

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

type Config struct {
	App      App
	HTTP     HTTP
	Database Database
	Auth     Auth
	Wallet   Wallet
	Log      Log
}

type App struct {
	Name string `env:"APP_NAME"`
	Env  string `env:"APP_ENV"`
}

type HTTP struct {
	Host               string        `env:"HTTP_HOST"`
	Port               int           `env:"HTTP_PORT"`
	GinMode            string        `env:"GIN_MODE"`
	ReadHeaderTimeout  time.Duration `env:"HTTP_READ_HEADER_TIMEOUT"`
	ReadTimeout        time.Duration `env:"HTTP_READ_TIMEOUT"`
	WriteTimeout       time.Duration `env:"HTTP_WRITE_TIMEOUT"`
	IdleTimeout        time.Duration `env:"HTTP_IDLE_TIMEOUT"`
	ShutdownTimeout    time.Duration `env:"HTTP_SHUTDOWN_TIMEOUT"`
	MaxBodyBytes       int64         `env:"HTTP_MAX_BODY_BYTES"`
	TrustedProxies     []string      `env:"HTTP_TRUSTED_PROXIES" envSeparator:","`
	CORSAllowedOrigins []string      `env:"HTTP_CORS_ALLOWED_ORIGINS" envSeparator:","`
}

func (h HTTP) Addr() string {
	return fmt.Sprintf("%s:%d", h.Host, h.Port)
}

type Database struct {
	Host               string        `env:"DB_HOST"`
	Port               int           `env:"DB_PORT"`
	User               string        `env:"DB_USER"`
	Password           string        `env:"DB_PASSWORD"`
	Name               string        `env:"DB_NAME"`
	SSLMode            string        `env:"DB_SSL_MODE"`
	MaxOpenConns       int           `env:"DB_MAX_OPEN_CONNS"`
	MaxIdleConns       int           `env:"DB_MAX_IDLE_CONNS"`
	ConnMaxLifetime    time.Duration `env:"DB_CONN_MAX_LIFETIME"`
	ConnMaxIdleTime    time.Duration `env:"DB_CONN_MAX_IDLE_TIME"`
	ConnectTimeout     time.Duration `env:"DB_CONNECT_TIMEOUT"`
	SlowQueryThreshold time.Duration `env:"DB_SLOW_QUERY_THRESHOLD"`
}

type Auth struct {
	JWTSecret       string        `env:"JWT_SECRET"`
	JWTIssuer       string        `env:"JWT_ISSUER"`
	JWTAudience     string        `env:"JWT_AUDIENCE"`
	AccessTokenTTL  time.Duration `env:"JWT_ACCESS_TOKEN_TTL"`
	RefreshTokenTTL time.Duration `env:"REFRESH_TOKEN_TTL"`
	BcryptCost      int           `env:"BCRYPT_COST"`
	RateLimitRPS    float64       `env:"AUTH_RATE_LIMIT_RPS"`
	RateLimitBurst  int           `env:"AUTH_RATE_LIMIT_BURST"`
	RateLimitTTL    time.Duration `env:"AUTH_RATE_LIMIT_TTL"`
}

type Wallet struct {
	SupportedCurrencies []string `env:"WALLET_SUPPORTED_CURRENCIES" envSeparator:","`
	MaxTopUpAmount      int64    `env:"WALLET_MAX_TOPUP_AMOUNT"`
	MaxTransferAmount   int64    `env:"WALLET_MAX_TRANSFER_AMOUNT"`
	MaxBalance          int64    `env:"WALLET_MAX_BALANCE"`
}

type Log struct {
	Level  string `env:"LOG_LEVEL"`
	Format string `env:"LOG_FORMAT"`
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("load .env file: %w", err)
	}

	var cfg Config
	if err := env.ParseWithOptions(&cfg, env.Options{RequiredIfNoDef: true}); err != nil {
		return nil, fmt.Errorf("parse environment: %w", err)
	}

	cfg.normalize()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) normalize() {
	c.HTTP.TrustedProxies = compact(c.HTTP.TrustedProxies)
	c.HTTP.CORSAllowedOrigins = compact(c.HTTP.CORSAllowedOrigins)
	c.Wallet.SupportedCurrencies = compact(c.Wallet.SupportedCurrencies)
	for i, currency := range c.Wallet.SupportedCurrencies {
		c.Wallet.SupportedCurrencies[i] = strings.ToUpper(currency)
	}
}

func (c *Config) Validate() error {
	var errs []error

	if !slices.Contains([]string{"development", "staging", "production"}, c.App.Env) {
		errs = append(errs, fmt.Errorf("APP_ENV must be one of development, staging, production"))
	}
	if !slices.Contains([]string{gin.DebugMode, gin.ReleaseMode, gin.TestMode}, c.HTTP.GinMode) {
		errs = append(errs, fmt.Errorf("GIN_MODE must be one of debug, release, test"))
	}
	if c.HTTP.Port <= 0 || c.HTTP.Port > 65535 {
		errs = append(errs, fmt.Errorf("HTTP_PORT must be between 1 and 65535"))
	}
	if c.HTTP.MaxBodyBytes <= 0 {
		errs = append(errs, fmt.Errorf("HTTP_MAX_BODY_BYTES must be positive"))
	}
	if c.HTTP.ReadHeaderTimeout <= 0 || c.HTTP.ReadTimeout <= 0 || c.HTTP.WriteTimeout <= 0 || c.HTTP.IdleTimeout <= 0 || c.HTTP.ShutdownTimeout <= 0 {
		errs = append(errs, fmt.Errorf("HTTP timeouts must be positive"))
	}

	if c.Database.Port <= 0 || c.Database.Port > 65535 {
		errs = append(errs, fmt.Errorf("DB_PORT must be between 1 and 65535"))
	}
	if c.Database.MaxOpenConns <= 0 {
		errs = append(errs, fmt.Errorf("DB_MAX_OPEN_CONNS must be positive"))
	}
	if c.Database.MaxIdleConns < 0 || c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		errs = append(errs, fmt.Errorf("DB_MAX_IDLE_CONNS must be between 0 and DB_MAX_OPEN_CONNS"))
	}
	if c.Database.ConnectTimeout <= 0 {
		errs = append(errs, fmt.Errorf("DB_CONNECT_TIMEOUT must be positive"))
	}

	if len(c.Auth.JWTSecret) < minJWTSecretLength {
		errs = append(errs, fmt.Errorf("JWT_SECRET must be at least %d characters", minJWTSecretLength))
	}
	if c.Auth.AccessTokenTTL <= 0 || c.Auth.RefreshTokenTTL <= c.Auth.AccessTokenTTL {
		errs = append(errs, fmt.Errorf("REFRESH_TOKEN_TTL must be greater than JWT_ACCESS_TOKEN_TTL and both must be positive"))
	}
	if c.Auth.BcryptCost < bcrypt.MinCost || c.Auth.BcryptCost > bcrypt.MaxCost {
		errs = append(errs, fmt.Errorf("BCRYPT_COST must be between %d and %d", bcrypt.MinCost, bcrypt.MaxCost))
	}
	if c.Auth.RateLimitRPS <= 0 || c.Auth.RateLimitBurst <= 0 || c.Auth.RateLimitTTL <= 0 {
		errs = append(errs, fmt.Errorf("AUTH_RATE_LIMIT_RPS, AUTH_RATE_LIMIT_BURST and AUTH_RATE_LIMIT_TTL must be positive"))
	}

	if len(c.Wallet.SupportedCurrencies) == 0 {
		errs = append(errs, fmt.Errorf("WALLET_SUPPORTED_CURRENCIES must not be empty"))
	}
	for _, currency := range c.Wallet.SupportedCurrencies {
		if !currencyPattern.MatchString(currency) {
			errs = append(errs, fmt.Errorf("WALLET_SUPPORTED_CURRENCIES contains invalid currency %q", currency))
		}
	}
	if c.Wallet.MaxTopUpAmount <= 0 || c.Wallet.MaxTransferAmount <= 0 || c.Wallet.MaxBalance <= 0 {
		errs = append(errs, fmt.Errorf("wallet amount limits must be positive"))
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(c.Log.Level)); err != nil {
		errs = append(errs, fmt.Errorf("LOG_LEVEL is invalid: %w", err))
	}
	if !slices.Contains([]string{"json", "text"}, c.Log.Format) {
		errs = append(errs, fmt.Errorf("LOG_FORMAT must be one of json, text"))
	}

	return errors.Join(errs...)
}

func compact(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
