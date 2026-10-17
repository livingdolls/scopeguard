-- +goose Up

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    email VARCHAR(320) NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT users_email_not_blank
                   CHECK ( LENGTH(BTRIM(email)) > 0 ),

    CONSTRAINT users_email_canonical
                   CHECK ( email = LOWER(BTRIM(email)) )
);

CREATE UNIQUE INDEX users_email_lower_unique
    ON users (LOWER(email));

CREATE TABLE organization_memberships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    organization_id UUID NOT NULL,
    user_id UUID NOT NULL,

    role VARCHAR(16) NOT NULL DEFAULT 'member',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT organization_memberships_org_fk
                                    FOREIGN KEY (organization_id)
                                      REFERENCES organizations(id)
                                      ON DELETE CASCADE ,

    CONSTRAINT organization_memberships_user_fk
                                      FOREIGN KEY (user_id)
                                      REFERENCES users(id)
                                      ON DELETE RESTRICT ,

    CONSTRAINT organization_memberships_role_check
                                      CHECK ( role IN ('owner', 'member') ),

    CONSTRAINT organization_memberships_unique_member
                                      UNIQUE (organization_id, user_id)
);

CREATE INDEX organization_memberships_user_idx
    ON organization_memberships (user_id);


-- +goose Down

DROP TABLE organization_memberships;
DROP TABLE users;