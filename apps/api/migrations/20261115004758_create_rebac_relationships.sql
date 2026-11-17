-- +goose Up

CREATE TABLE rebac_relationships (
    organization_id UUID NOT NULL,
    resource_id UUID NOT NULL,
    subject_membership_id UUID NOT NULL,

    relation VARCHAR(16) NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT rebac_relationships_pkey
        PRIMARY KEY (
            organization_id,
            resource_id,
            subject_membership_id,
            relation
        ),

    CONSTRAINT rebac_relationships_relation_check
        CHECK (
            relation IN ('viewer', 'editor')
        ),

    CONSTRAINT rebac_relationships_resource_fk
        FOREIGN KEY (
            organization_id,
            resource_id
        )
        REFERENCES rebac_resources (
            organization_id,
            id
        )
        ON DELETE CASCADE,

    CONSTRAINT rebac_relationships_subject_fk
        FOREIGN KEY (
            organization_id,
            subject_membership_id
        )
        REFERENCES organization_memberships (
            organization_id,
            id
        )
        ON DELETE CASCADE
);

CREATE INDEX rebac_relationships_subject_idx
    ON rebac_relationships (
        organization_id,
        subject_membership_id,
        resource_id
    );

-- +goose Down

DROP TABLE rebac_relationships;