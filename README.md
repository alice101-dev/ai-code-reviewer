# Wallet Service

REST API for digital wallets: user authentication, multi-currency wallets, top-ups and peer-to-peer transfers, backed by a double-entry ledger in PostgreSQL.

## Features

- JWT access tokens with rotating, hashed refresh tokens and reuse detection
- One wallet per user per currency, with configurable supported currencies
- Idempotent top-ups and transfers through the `Idempotency-Key` header
- Pessimistic row locking with deterministic lock order for concurrent transfers
- Double-entry ledger with running balances per wallet
- Per-IP rate limiting on authentication endpoints, request body limits and security headers
- Structured JSON logging with request IDs, liveness and readiness probes, graceful shutdown

## Tech Stack

| Concern    | Choice                                   |
|------------|------------------------------------------|
| Language   | Go 1.27                                  |
| HTTP       | Gin                                      |
| Database   | PostgreSQL 18, GORM with the pgx driver  |
| Migrations | golang-migrate                           |
| Auth       | golang-jwt (HS256), bcrypt               |
| Tooling    | Docker, golangci-lint, GitHub Actions    |

## Getting Started

### Prerequisites

- Go 1.27+
- Docker with Compose v2

### Run with Docker Compose

```bash
cp .env.example .env
make up
```

Set `DB_PASSWORD` and `JWT_SECRET` in `.env` before starting. `make up` starts PostgreSQL, applies migrations and starts the API on `HTTP_PORT`.

### Run the API on the host

```bash
make db-up
make migrate-up
make run
```

## Configuration

All configuration is read from environment variables. A `.env` file in the working directory is loaded automatically when present; variables already set in the environment take precedence. Every variable is required, and the service refuses to start on missing or invalid values.

| Variable | Description |
|---|---|
| `APP_NAME`, `APP_ENV` | Service name and environment (`development`, `staging`, `production`) |
| `HTTP_HOST`, `HTTP_PORT` | Listen address |
| `GIN_MODE` | `debug`, `release` or `test` |
| `HTTP_READ_HEADER_TIMEOUT`, `HTTP_READ_TIMEOUT`, `HTTP_WRITE_TIMEOUT`, `HTTP_IDLE_TIMEOUT` | HTTP server timeouts |
| `HTTP_SHUTDOWN_TIMEOUT` | Grace period for in-flight requests on shutdown |
| `HTTP_MAX_BODY_BYTES` | Maximum request body size |
| `HTTP_TRUSTED_PROXIES` | Comma-separated proxy IPs/CIDRs allowed to set `X-Forwarded-For` |
| `HTTP_CORS_ALLOWED_ORIGINS` | Comma-separated origins; CORS is disabled when empty |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSL_MODE` | PostgreSQL connection |
| `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`, `DB_CONN_MAX_LIFETIME`, `DB_CONN_MAX_IDLE_TIME` | Connection pool |
| `DB_CONNECT_TIMEOUT` | Connect and readiness ping timeout |
| `DB_SLOW_QUERY_THRESHOLD` | Queries slower than this are logged as warnings |
| `JWT_SECRET` | HMAC signing key, at least 32 characters |
| `JWT_ISSUER`, `JWT_AUDIENCE`, `JWT_ACCESS_TOKEN_TTL` | Access token claims and lifetime |
| `REFRESH_TOKEN_TTL` | Refresh token lifetime |
| `BCRYPT_COST` | Password hashing cost |
| `AUTH_RATE_LIMIT_RPS`, `AUTH_RATE_LIMIT_BURST`, `AUTH_RATE_LIMIT_TTL` | Per-IP rate limit for `/auth` endpoints |
| `WALLET_SUPPORTED_CURRENCIES` | Comma-separated ISO 4217 codes |
| `WALLET_MAX_TOPUP_AMOUNT`, `WALLET_MAX_TRANSFER_AMOUNT`, `WALLET_MAX_BALANCE` | Amount limits in minor units |
| `LOG_LEVEL`, `LOG_FORMAT` | `debug`/`info`/`warn`/`error` and `json`/`text` |
| `TEST_DATABASE_URL` | PostgreSQL URL used by integration tests |

## API

Base path: `/api/v1`. All amounts are integers in the currency's minor unit.

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/auth/register` | – | Create an account |
| `POST` | `/auth/login` | – | Exchange credentials for a token pair |
| `POST` | `/auth/refresh` | – | Rotate a refresh token |
| `POST` | `/auth/logout` | Bearer | Revoke a refresh token |
| `GET` | `/users/me` | Bearer | Current user profile |
| `POST` | `/wallets` | Bearer | Open a wallet for a currency |
| `GET` | `/wallets` | Bearer | List own wallets |
| `GET` | `/wallets/{id}` | Bearer | Get a wallet |
| `GET` | `/wallets/{id}/transactions` | Bearer | Paginated ledger entries (`page`, `page_size`) |
| `POST` | `/wallets/{id}/topups` | Bearer | Top up a wallet, requires `Idempotency-Key` |
| `POST` | `/transfers` | Bearer | Transfer between wallets, requires `Idempotency-Key` |
| `GET` | `/healthz`, `/readyz` | – | Liveness and readiness probes (outside `/api/v1`) |

### Example

```bash
curl -s -X POST localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"jane@example.com","password":"correct-horse-battery","full_name":"Jane Doe"}'

TOKEN=$(curl -s -X POST localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"jane@example.com","password":"correct-horse-battery"}' | jq -r .data.access_token)

WALLET=$(curl -s -X POST localhost:8080/api/v1/wallets \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"currency":"IDR"}' | jq -r .data.id)

curl -s -X POST localhost:8080/api/v1/wallets/$WALLET/topups \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -H "Idempotency-Key: $(uuidgen)" \
  -d '{"amount":150000}'
```

### Idempotency

`POST /wallets/{id}/topups` and `POST /transfers` require an `Idempotency-Key` header of 8–64 characters (`A-Z`, `a-z`, `0-9`, `-`, `_`). Keys are scoped per user. Repeating a request with the same key and payload returns the original transaction with `200 OK` and `Idempotent-Replayed: true`; reusing a key with a different payload returns `409 Conflict`.

### Errors

```json
{
  "error": {
    "code": "invalid_argument",
    "message": "request validation failed",
    "request_id": "6f1c1f0e-8a59-4c1b-9d0c-5c8f3b7b2a10",
    "details": [{ "field": "email", "rule": "email" }]
  }
}
```

| Code | HTTP status |
|---|---|
| `invalid_argument` | 400 |
| `unauthenticated` | 401 |
| `permission_denied` | 403 |
| `not_found` | 404 |
| `conflict` | 409 |
| `payload_too_large` | 413 |
| `unprocessable_entity` | 422 |
| `rate_limited` | 429 |
| `internal` | 500 |

## Development

```bash
make test
make lint
make migrate-create name=add_wallet_limits
```

Integration tests run against `TEST_DATABASE_URL`. Each test creates an isolated schema, applies the migrations and drops the schema afterwards. Tests that need a database are skipped when the variable is not set.

## Project Layout

```
cmd/api                 entrypoint
internal/app            dependency wiring and HTTP server lifecycle
internal/config         environment configuration and validation
internal/database       connection pool and transaction management
internal/httpapi        router, handlers, middleware and response encoding
internal/service        business logic for auth, wallets and payments
internal/repository     GORM data access
internal/model          persistence models
internal/security       password hashing, JWT and refresh tokens
migrations              SQL schema migrations
```
