-- +goose Up

CREATE TABLE permissions (
    code VARCHAR(127) PRIMARY KEY,

    resource VARCHAR(63) NOT NULL,
    action VARCHAR(63) NOT NULL,

    description VARCHAR(255) NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT permissions_resource_format
        CHECK (
            resource ~ '^[a-z][a-z0-9_]*$'
        ),

    CONSTRAINT permissions_action_format
        CHECK (
            action ~ '^[a-z][a-z0-9_]*$'
        ),

    CONSTRAINT permissions_code_matches_parts
        CHECK (
            code = resource || ':' || action
        ),

    CONSTRAINT permissions_description_not_blank
        CHECK (
            LENGTH(BTRIM(description)) > 0
        )
);

INSERT INTO permissions (code, resource, action, description)
VALUES
    (
        'organization:read',
        'organization', 'read',
        'View organization details'
    ),
    (
        'organization:update',
        'organization', 'update',
        'Update organization details'
    ),
    (
        'organization:delete',
        'organization', 'delete',
        'Delete organization'
    ),
    (
        'member:read',
        'member', 'read',
        'View organization members'
    ),
    (
        'member:invite',
        'member', 'invite',
        'Invite organization members'
    ),
    (
        'member:remove',
        'member', 'remove',
        'Remove organization members'
    ),
    (
        'member:role_assign',
        'member', 'role_assign',
        'Assign roles to organization members'
    ),
    (
        'role:read',
        'role', 'read',
        'View organization roles'
    ),
    (
        'role:create',
        'role', 'create',
        'Create organization roles'
    ),
    (
        'role:update',
        'role', 'update',
        'Update organization roles'
    ),
    (
        'role:delete',
        'role', 'delete',
        'Delete organization roles'
    ),
    (
        'permission:read',
        'permission', 'read',
        'View available permissions'
    );

-- +goose Down

DROP TABLE permissions;