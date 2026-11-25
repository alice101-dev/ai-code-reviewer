package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validEnv() map[string]string {
	return map[string]string{
		"APP_NAME":                    "wallet-service",
		"APP_ENV":                     "development",
		"HTTP_HOST":                   "127.0.0.1",
		"HTTP_PORT":                   "8080",
		"GIN_MODE":                    "release",
		"HTTP_READ_HEADER_TIMEOUT":    "5s",
		"HTTP_READ_TIMEOUT":           "15s",
		"HTTP_WRITE_TIMEOUT":          "15s",
		"HTTP_IDLE_TIMEOUT":           "60s",
		"HTTP_SHUTDOWN_TIMEOUT":       "20s",
		"HTTP_MAX_BODY_BYTES":         "1048576",
		"HTTP_TRUSTED_PROXIES":        "",
		"HTTP_CORS_ALLOWED_ORIGINS":   " https://app.example.com , ",
		"DB_HOST":                     "localhost",
		"DB_PORT":                     "5432",
		"DB_USER":                     "wallet",
		"DB_PASSWORD":                 "secret",
		"DB_NAME":                     "wallet",
		"DB_SSL_MODE":                 "disable",
		"DB_MAX_OPEN_CONNS":           "25",
		"DB_MAX_IDLE_CONNS":           "10",
		"DB_CONN_MAX_LIFETIME":        "30m",
		"DB_CONN_MAX_IDLE_TIME":       "5m",
		"DB_CONNECT_TIMEOUT":          "5s",
		"DB_SLOW_QUERY_THRESHOLD":     "200ms",
		"JWT_SECRET":                  "0123456789abcdef0123456789abcdef",
		"JWT_ISSUER":                  "wallet-service",
		"JWT_AUDIENCE":                "wallet-api",
		"JWT_ACCESS_TOKEN_TTL":        "15m",
		"REFRESH_TOKEN_TTL":           "720h",
		"BCRYPT_COST":                 "12",
		"AUTH_RATE_LIMIT_RPS":         "1",
		"AUTH_RATE_LIMIT_BURST":       "5",
		"AUTH_RATE_LIMIT_TTL":         "10m",
		"WALLET_SUPPORTED_CURRENCIES": "idr, usd",
		"WALLET_MAX_TOPUP_AMOUNT":     "1000",
		"WALLET_MAX_TRANSFER_AMOUNT":  "500",
		"WALLET_MAX_BALANCE":          "10000",
		"LOG_LEVEL":                   "info",
		"LOG_FORMAT":                  "json",
	}
}

func setEnv(t *testing.T, env map[string]string) {
	t.Helper()
	t.Chdir(t.TempDir())
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func TestLoad(t *testing.T) {
	setEnv(t, validEnv())

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "127.0.0.1:8080", cfg.HTTP.Addr())
	assert.Equal(t, 15*time.Minute, cfg.Auth.AccessTokenTTL)
	assert.Equal(t, []string{"IDR", "USD"}, cfg.Wallet.SupportedCurrencies)
	assert.Equal(t, []string{"https://app.example.com"}, cfg.HTTP.CORSAllowedOrigins)
	assert.Empty(t, cfg.HTTP.TrustedProxies)
}

func TestLoadFailsWhenVariableIsMissing(t *testing.T) {
	env := validEnv()
	delete(env, "JWT_SECRET")
	setEnv(t, env)
	t.Setenv("JWT_SECRET", "")
	require.NoError(t, os.Unsetenv("JWT_SECRET"))

	_, err := Load()
	require.ErrorContains(t, err, "JWT_SECRET")
}

func TestLoadValidation(t *testing.T) {
	tests := map[string]struct {
		key, value, want string
	}{
		"short jwt secret":         {"JWT_SECRET", "too-short", "JWT_SECRET must be at least"},
		"invalid app env":          {"APP_ENV", "prod", "APP_ENV"},
		"bcrypt cost too high":     {"BCRYPT_COST", "40", "BCRYPT_COST"},
		"refresh shorter than jwt": {"REFRESH_TOKEN_TTL", "1m", "REFRESH_TOKEN_TTL"},
		"idle above open conns":    {"DB_MAX_IDLE_CONNS", "100", "DB_MAX_IDLE_CONNS"},
		"invalid currency":         {"WALLET_SUPPORTED_CURRENCIES", "IDR,RUPIAH", "invalid currency"},
		"invalid log level":        {"LOG_LEVEL", "verbose", "LOG_LEVEL"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			env := validEnv()
			env[tt.key] = tt.value
			setEnv(t, env)

			_, err := Load()
			require.ErrorContains(t, err, tt.want)
		})
	}
}
