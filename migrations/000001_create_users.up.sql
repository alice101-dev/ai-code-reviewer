CREATE TABLE users (
    id            UUID         PRIMARY KEY DEFAULT uuidv7(),
    email         VARCHAR(255) NOT NULL,
    password_hash TEXT         NOT NULL,
    full_name     VARCHAR(100) NOT NULL,
    status        VARCHAR(20)  NOT NULL DEFAULT 'active',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT users_email_key UNIQUE (email),
    CONSTRAINT users_email_lowercase CHECK (email = lower(email)),
    CONSTRAINT users_status_check CHECK (status IN ('active', 'suspended'))
);
