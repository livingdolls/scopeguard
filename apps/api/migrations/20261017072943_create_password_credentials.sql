-- +goose Up

CREATE TABLE user_password_credentials (
    user_id UUID PRIMARY KEY,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT user_password_credentials_user_fk
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON DELETE CASCADE,

    CONSTRAINT user_password_credentials_hash_not_blank
        CHECK ( LENGTH(BTRIM(password_hash)) > 0 )
);

-- +goose Down

DROP TABLE user_password_credentials;