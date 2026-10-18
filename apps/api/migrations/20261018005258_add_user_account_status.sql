-- +goose Up

ALTER TABLE users
    ADD COLUMN status VARCHAR(24)
        NOT NULL DEFAULT 'pending_verification',

    ADD COLUMN email_verified_at TIMESTAMPTZ,

    ADD CONSTRAINT users_status_check
        CHECK (
            STATUS IN (
                'pending_verification',
                'active',
                'suspended'
                )
            ),

    ADD CONSTRAINT users_active_requires_verified_email
        CHECK (
            status <> 'active'
            OR email_verified_at IS NOT NULL
        );

-- +goose Down

ALTER TABLE users
    DROP COLUMN email_verified_at,
    DROP COLUMN status;