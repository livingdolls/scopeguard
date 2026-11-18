-- +goose Up
INSERT INTO permissions (
    code,
    resource,
    action,
    description
)
VALUES
    (
        'resource:access',
     'resource',
     'access',
     'Access resources through relationship policies'
    ),
    (
     'resource:admin',
     'resource',
     'admin',
     'Administer organization resources'
    );

-- Backfill system roles from existing  organizations
WITH default_policy(role_slug, permission_code) AS (
    VALUES
        ('owner', 'resource:access'),
        ('owner', 'resource:admin'),
        ('member', 'resource:access')
)
INSERT INTO role_permissions (
    organization_id,
    role_id,
    permission_code
)
SELECT
    r.organization_id,
    r.id,
    p.permission_code
FROM roles AS r
JOIN default_policy AS p
    ON p.role_slug = r.slug
WHERE r.is_system = TRUE
ON CONFLICT (
    organization_id,
    role_id,
    permission_code
)
DO NOTHING;

-- +goose Down

DELETE FROM role_permissions
WHERE permission_code IN (
    'resource:access',
    'resource:admin'
);

DELETE FROM permissions
WHERE code IN (
    'resource:access',
    'resource:admin'
);