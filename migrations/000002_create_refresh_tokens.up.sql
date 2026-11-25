CREATE TABLE refresh_tokens (
    id         UUID        PRIMARY KEY DEFAULT uuidv7(),
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash CHAR(64)    NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT refresh_tokens_token_hash_key UNIQUE (token_hash)
);

CREATE INDEX refresh_tokens_user_id_active_idx ON refresh_tokens (user_id) WHERE revoked_at IS NULL;
