//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestReBACRelationshipConstraints(t *testing.T) {
	pool, _, ownerUserID := setupVerificationUser(t)

	tenantA := createRoleTestOrganization(t, pool, ownerUserID)
	tenantB := createRoleTestOrganization(t, pool, ownerUserID)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	getMembership := func(organizationID uuid.UUID) uuid.UUID {
		t.Helper()

		var id uuid.UUID
		err := pool.QueryRow(ctx,
			`SELECT id FROM organization_memberships
             WHERE organization_id = $1 AND user_id = $2`,
			organizationID,
			ownerUserID,
		).Scan(&id)

		if err != nil {
			t.Fatal(err)
		}
		return id
	}

	createProject := func(
		organizationID uuid.UUID,
		ownerID uuid.UUID,
	) uuid.UUID {
		t.Helper()

		var id uuid.UUID
		err := pool.QueryRow(ctx,
			`INSERT INTO rebac_resources (
                organization_id,
                resource_type,
                external_key,
                owner_membership_id
            )
            VALUES ($1, 'project', $2, $3)
            RETURNING id`,
			organizationID,
			uuid.NewString(),
			ownerID,
		).Scan(&id)

		if err != nil {
			t.Fatal(err)
		}
		return id
	}

	membershipA := getMembership(tenantA)
	membershipB := getMembership(tenantB)

	resourceA := createProject(tenantA, membershipA)
	resourceB := createProject(tenantB, membershipB)

	const insert = `
        INSERT INTO rebac_relationships (
            organization_id,
            resource_id,
            subject_membership_id,
            relation
        ) VALUES ($1, $2, $3, $4)
    `

	// Valid viewer relationship.
	_, err := pool.Exec(
		ctx, insert,
		tenantA, resourceA, membershipA, "viewer",
	)
	if err != nil {
		t.Fatalf("valid relationship rejected: %v", err)
	}

	tests := []struct {
		name       string
		org        uuid.UUID
		resource   uuid.UUID
		subject    uuid.UUID
		relation   string
		constraint string
		code       string
	}{
		{
			name:       "cross tenant resource",
			org:        tenantA,
			resource:   resourceB,
			subject:    membershipA,
			relation:   "viewer",
			constraint: "rebac_relationships_resource_fk",
			code:       "23503",
		},
		{
			name:       "cross tenant subject",
			org:        tenantA,
			resource:   resourceA,
			subject:    membershipB,
			relation:   "viewer",
			constraint: "rebac_relationships_subject_fk",
			code:       "23503",
		},
		{
			name:       "owner relation forbidden",
			org:        tenantA,
			resource:   resourceA,
			subject:    membershipA,
			relation:   "owner",
			constraint: "rebac_relationships_relation_check",
			code:       "23514",
		},
		{
			name:       "duplicate tuple",
			org:        tenantA,
			resource:   resourceA,
			subject:    membershipA,
			relation:   "viewer",
			constraint: "rebac_relationships_pkey",
			code:       "23505",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(
				ctx,
				insert,
				tc.org,
				tc.resource,
				tc.subject,
				tc.relation,
			)

			var pgErr *pgconn.PgError

			if !errors.As(err, &pgErr) ||
				pgErr.Code != tc.code ||
				pgErr.ConstraintName != tc.constraint {
				t.Fatalf(
					"expected %s (%s), got %v",
					tc.constraint,
					tc.code,
					err,
				)
			}
		})
	}
}
