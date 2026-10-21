-- +goose Up

ALTER TABLE email_outbox
    ADD COLUMN lease_id UUID;

ALTER TABLE email_outbox
    ADD CONSTRAINT email_outbox_processing_lease_check
    CHECK (
        (
            status = 'processing'
            AND lease_id IS NOT NULL
            AND locked_until IS NOT NULL
        )
        OR
        (
            status <> 'processing'
            AND lease_id IS NULL
            AND locked_until IS NULL
        )
    );

-- +goose Down

ALTER TABLE email_outbox
    DROP CONSTRAINT email_outbox_processing_lease_check,
    DROP COLUMN lease_id;