-- +goose Up

CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    name VARCHAR(160) NOT NULL,
    slug VARCHAR(63) NOT NULL UNIQUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT organizations_name_not_blank
                        CHECK ( LENGTH(BTRIM(name)) > 0 ),
    CONSTRAINT organizations_slug_format
                           CHECK (
                                slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'
                            )
);

-- +goose Down

DROP TABLE organizations;
