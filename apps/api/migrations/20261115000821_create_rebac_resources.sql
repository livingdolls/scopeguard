-- +goose Up
CREATE TABLE rebac_resources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    organization_id UUID NOT NULL,
    resource_type VARCHAR(32) NOT NULL,
    external_key VARCHAR(128) NOT NULL,

    parent_resource_id UUID,
    owner_membership_id UUID NOT NULL,

    version BIGINT NOT NULL DEFAULT 1,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT rebac_resources_org_fk
        FOREIGN KEY (organization_id)
        REFERENCES organizations(id)
        ON DELETE CASCADE,

    CONSTRAINT rebac_resources_org_id_unique
        UNIQUE (organization_id, id),

    CONSTRAINT rebac_resources_identity_unique
        UNIQUE(
            organization_id,
            resource_type,
            external_key
        ),

    CONSTRAINT rebac_resources_type_check
        CHECK (
            resource_type IN (
                'project',
                'folder',
                'document'
            )
        ),

    CONSTRAINT rebac_resources_external_key_check
        CHECK (
            external_key ~
            '^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$'
        ),

    CONSTRAINT rebac_resources_parent_shape_check
        CHECK (
            (
                resource_type = 'project'
                AND parent_resource_id IS NULL
            )
            OR
            (
                resource_type IN ('folder', 'document')
                AND parent_resource_id IS NOT NULL
            )
        ),

    CONSTRAINT rebac_resources_not_self_parent_check
        CHECK (
            parent_resource_id IS NULL
            OR parent_resource_id <> id
        ),

    CONSTRAINT rebac_resources_version_check
        CHECK ( version >= 1 ),

    CONSTRAINT rebac_resources_time_check
        CHECK ( updated_at >= created_at ),

    CONSTRAINT rebac_resources_owner_fk
        FOREIGN KEY (
            organization_id,
            owner_membership_id
        )
        REFERENCES organization_memberships (
            organization_id,
            id
        )
        ON DELETE NO ACTION
        DEFERRABLE INITIALLY DEFERRED,

    CONSTRAINT rebac_resources_parent_fk
        FOREIGN KEY (
            organization_id,
            parent_resource_id
        )
        REFERENCES rebac_resources (
            organization_id,
            id
        )
        ON DELETE NO ACTION
        DEFERRABLE INITIALLY DEFERRED
);

CREATE INDEX rebac_resources_parent_idx
    ON rebac_resources (
        organization_id,
        parent_resource_id
    )
    WHERE parent_resource_id IS NOT NULL;

CREATE INDEX rebac_resources_owner_idx
    ON rebac_resources (
        organization_id,
        owner_membership_id
    );


-- +goose Down

DROP TABLE rebac_resources;