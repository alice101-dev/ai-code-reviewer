CREATE TABLE wallets (
    id         UUID        PRIMARY KEY DEFAULT uuidv7(),
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    currency   CHAR(3)     NOT NULL,
    balance    BIGINT      NOT NULL DEFAULT 0,
    status     VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT wallets_user_currency_key UNIQUE (user_id, currency),
    CONSTRAINT wallets_balance_non_negative CHECK (balance >= 0),
    CONSTRAINT wallets_currency_format CHECK (currency ~ '^[A-Z]{3}$'),
    CONSTRAINT wallets_status_check CHECK (status IN ('active', 'frozen', 'closed'))
);
