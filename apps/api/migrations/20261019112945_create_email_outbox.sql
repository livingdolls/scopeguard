-- +goose Up

CREATE TABLE email_outbox (
    id UUID PRIMARY KEY,

    verification_token_id UUID NOT NULL UNIQUE,
    event_type  VARCHAR(64) NOT NULL DEFAULT 'identity.email_verification',
    encrypted_payload BYTEA NOT NULL,
    encrypted_key_id VARCHAR(32) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_until TIMESTAMPTZ,
    sent_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT email_outbox_verification_fk
        FOREIGN KEY (verification_token_id)
        REFERENCES email_verification_tokens(id)
        ON DELETE CASCADE,

    CONSTRAINT email_outbox_event_type_check
        CHECK ( event_type = 'identity.email_verification' ),

    CONSTRAINT email_outbox_status_check
        CHECK ( status IN (
            'pending',
            'processing',
            'sent',
            'dead'
        )),

    CONSTRAINT email_outbox_attempts_check
        CHECK ( attempts >= 0 ),

    CONSTRAINT email_outbox_payload_not_empty
        CHECK ( OCTET_LENGTH(encrypted_payload) > 28 ),

    CONSTRAINT email_outbox_key_not_blank
        CHECK ( LENGTH(BTRIM(encrypted_key_id)) > 0 )
);

CREATE INDEX email_outbox_dispatch_idx
    ON email_outbox (available_at, created_at)
    WHERE status = 'pending';

CREATE INDEX email_outbox_lease_idx
    ON email_outbox (locked_until)
    WHERE status = 'processing';

-- +goose Down

DROP TABLE email_outbox;