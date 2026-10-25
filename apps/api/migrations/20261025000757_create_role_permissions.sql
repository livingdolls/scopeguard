-- +goose Up

CREATE TABLE role_permissions (
    organization_id UUID NOT NULL,
    role_id UUID NOT NULL,
    permission_code VARCHAR(127) NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT role_permissions_pk
        PRIMARY KEY (
            organization_id,
            role_id,
            permission_code
        ),

    CONSTRAINT role_permissions_role_fk
        FOREIGN KEY (organization_id, role_id)
        REFERENCES roles (organization_id, id)
        ON DELETE CASCADE,

    CONSTRAINT role_permissions_permission_fk
        FOREIGN KEY (permission_code)
        REFERENCES permissions (code)
        ON DELETE RESTRICT
);

CREATE INDEX role_permissions_permission_code_idx
    ON role_permissions (permission_code);

-- Backfill existing system roles.
-- Keep this explicit policy synchronized with
-- domain.DefaultSystemRolePermissions.
WITH default_policy(role_slug, permission_code) AS (
    VALUES
        ('owner', 'organization:read'),
        ('owner', 'organization:update'),
        ('owner', 'organization:delete'),

        ('owner', 'member:read'),
        ('owner', 'member:invite'),
        ('owner', 'member:remove'),
        ('owner', 'member:role_assign'),

        ('owner', 'role:read'),
        ('owner', 'role:create'),
        ('owner', 'role:update'),
        ('owner', 'role:delete'),

        ('owner', 'permission:read'),

        ('member', 'organization:read'),
        ('member', 'member:read'),
        ('member', 'role:read'),
        ('member', 'permission:read')
)
INSERT INTO role_permissions (
    organization_id,
    role_id,
    permission_code
)
SELECT
    r.organization_id,
    r.id,
    policy.permission_code
FROM roles AS r
JOIN default_policy as policy
    ON policy.role_slug = r.slug
WHERE r.is_system = true;

-- +goose Down

DROP TABLE role_permissions;
