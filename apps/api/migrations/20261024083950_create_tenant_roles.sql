-- +goose Up

-- Role definitions belong to one organization
CREATE TABLE roles (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL,

    name VARCHAR(80) NOT NULL,
    slug VARCHAR(63) NOT NULL,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT roles_organization_fk
        FOREIGN KEY (organization_id)
        REFERENCES organizations(id)
        ON DELETE CASCADE,

    CONSTRAINT roles_organization_slug_unique
        UNIQUE (organization_id, slug),

    CONSTRAINT roles_organization_id_unique
        UNIQUE (organization_id, id),

    CONSTRAINT roles_name_valid
        CHECK (
            LENGTH(BTRIM(name)) > 0
            AND name = BTRIM(name)
        ),

    CONSTRAINT roles_slug_format
        CHECK (
            slug ~ '^[a-z][a-z0-9_]*$'
        ),

    CONSTRAINT roles_system_slug_check
        CHECK (
            (
                is_system = TRUE
                AND slug IN ('owner', 'member')
            )
            OR
            (
                is_system = FALSE
                AND slug NOT IN ('owner', 'member')
            )
        )
);

-- required for composite membership foreign keys
ALTER TABLE organization_memberships
    ADD CONSTRAINT organization_memberships_org_id_unique
    UNIQUE (organization_id, id);

-- Role assignments share the tenant ID of both parents
CREATE TABLE membership_roles (
    organization_id UUID NOT NULL,
    membership_id UUID NOT NULL,
    role_id UUID NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT membership_roles_pk
        PRIMARY KEY (membership_id, role_id),

    CONSTRAINT membership_roles_membership_fk
        FOREIGN KEY (organization_id, membership_id)
        REFERENCES organization_memberships (organization_id, id)
        ON DELETE CASCADE,

    CONSTRAINT membership_roles_role_fk
        FOREIGN KEY (organization_id, role_id)
        REFERENCES roles (organization_id, id)
        ON DELETE NO ACTION
        DEFERRABLE INITIALLY DEFERRED
);

CREATE INDEX membership_roles_roles_organization_idx
    ON membership_roles (organization_id);

CREATE INDEX membership_roles_role_idx
    ON membership_roles (role_id);

-- Backfill owner and member for existing organizations
INSERT INTO roles (id, organization_id, name, slug, is_system)
SELECT
    gen_random_uuid(),
    o.id,
    predefined.name,
    predefined.slug,
    TRUE
FROM organizations o
CROSS JOIN (
    VALUES ('Owner', 'owner'),
           ('Member', 'member')
) AS predefined(name, slug);

-- Backfill existing membership assignments
INSERT INTO membership_roles (organization_id, membership_id, role_id)
SELECT m.organization_id, m.id, r.id
FROM organization_memberships m
JOIN roles r
    ON r.organization_id = m.organization_id
    AND r.slug = m.role;


-- +goose Down

DROP TABLE membership_roles;

ALTER TABLE organization_memberships
    DROP CONSTRAINT organization_memberships_org_id_unique;

DROP TABLE roles;
