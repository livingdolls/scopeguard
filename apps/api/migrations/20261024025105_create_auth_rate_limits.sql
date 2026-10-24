-- +goose Up

CREATE TABLE auth_rate_limits (
    scope VARCHAR(64) NOT NULL,
    identity_hash CHAR(64) NOT NULL,
    hits INTEGER NOT NULL,
    reset_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (scope, identity_hash),

    CONSTRAINT auth_rate_limits_hits_check
        CHECK ( hits > 0 )
);

CREATE INDEX auth_rate_limits_reset_idx
    ON auth_rate_limits (reset_at);

-- +goose Down

DROP TABLE auth_rate_limits;