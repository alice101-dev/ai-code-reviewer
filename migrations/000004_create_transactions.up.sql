CREATE TABLE transactions (
    id                    UUID         PRIMARY KEY DEFAULT uuidv7(),
    user_id               UUID         NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    type                  VARCHAR(20)  NOT NULL,
    source_wallet_id      UUID         REFERENCES wallets (id) ON DELETE RESTRICT,
    destination_wallet_id UUID         NOT NULL REFERENCES wallets (id) ON DELETE RESTRICT,
    amount                BIGINT       NOT NULL,
    currency              CHAR(3)      NOT NULL,
    description           VARCHAR(255) NOT NULL DEFAULT '',
    idempotency_key       VARCHAR(64)  NOT NULL,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT transactions_user_idempotency_key UNIQUE (user_id, idempotency_key),
    CONSTRAINT transactions_amount_positive CHECK (amount > 0),
    CONSTRAINT transactions_type_check CHECK (type IN ('topup', 'transfer')),
    CONSTRAINT transactions_source_wallet_check CHECK (
        (type = 'topup' AND source_wallet_id IS NULL)
        OR (type = 'transfer' AND source_wallet_id IS NOT NULL AND source_wallet_id <> destination_wallet_id)
    )
);

CREATE INDEX transactions_source_wallet_id_idx ON transactions (source_wallet_id) WHERE source_wallet_id IS NOT NULL;
CREATE INDEX transactions_destination_wallet_id_idx ON transactions (destination_wallet_id);

CREATE TABLE ledger_entries (
    id             UUID        PRIMARY KEY DEFAULT uuidv7(),
    transaction_id UUID        NOT NULL REFERENCES transactions (id) ON DELETE RESTRICT,
    wallet_id      UUID        NOT NULL REFERENCES wallets (id) ON DELETE RESTRICT,
    direction      VARCHAR(6)  NOT NULL,
    amount         BIGINT      NOT NULL,
    balance_after  BIGINT      NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT ledger_entries_direction_check CHECK (direction IN ('debit', 'credit')),
    CONSTRAINT ledger_entries_amount_positive CHECK (amount > 0),
    CONSTRAINT ledger_entries_balance_after_non_negative CHECK (balance_after >= 0)
);

CREATE INDEX ledger_entries_wallet_id_created_at_idx ON ledger_entries (wallet_id, created_at DESC, id DESC);
CREATE INDEX ledger_entries_transaction_id_idx ON ledger_entries (transaction_id);
