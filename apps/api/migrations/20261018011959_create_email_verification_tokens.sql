-- +goose Up

CREATE TABLE email_verification_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL,
    token_hash BYTEA NOT NULL UNIQUE,

    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT email_verification_tokens_user_fk
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE,

    CONSTRAINT email_verification_tokens_hash_length
        CHECK ( OCTET_LENGTH(token_hash) = 32 ),

    CONSTRAINT email_verification_tokens_expiry
        CHECK ( expires_at > created_at )
);

CREATE INDEX email_verification_tokens_pending_user_idx
    ON email_verification_tokens (user_id)
    WHERE consumed_at IS NULL;

-- +goose Down

DROP TABLE email_verification_tokens;