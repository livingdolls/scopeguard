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

func TestReBACResourceConstraints(t *testing.T) {
	pool, _, ownerUserID := setupVerificationUser(t)

	tenantA := createRoleTestOrganization(t, pool, ownerUserID)
	tenantB := createRoleTestOrganization(t, pool, ownerUserID)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	// Get the owner's membership within each organization.
	membership := func(organizationID uuid.UUID) uuid.UUID {
		t.Helper()

		var id uuid.UUID
		err := pool.QueryRow(
			ctx,
			`SELECT id
             FROM organization_memberships
             WHERE organization_id = $1 AND user_id = $2`,
			organizationID,
			ownerUserID,
		).Scan(&id)

		if err != nil {
			t.Fatal(err)
		}
		return id
	}

	ownerA := membership(tenantA)
	ownerB := membership(tenantB)

	// A valid root project.
	var projectA uuid.UUID

	err := pool.QueryRow(
		ctx,
		`INSERT INTO rebac_resources (
            organization_id,
            resource_type,
            external_key,
            owner_membership_id
        )
        VALUES ($1, 'project', 'website', $2)
        RETURNING id`,
		tenantA,
		ownerA,
	).Scan(&projectA)

	if err != nil {
		t.Fatal(err)
	}

	// A valid folder inside the project.
	var version int64

	err = pool.QueryRow(
		ctx,
		`INSERT INTO rebac_resources (
            organization_id,
            resource_type,
            external_key,
            parent_resource_id,
            owner_membership_id
        )
        VALUES ($1, 'folder', 'assets', $2, $3)
        RETURNING version`,
		tenantA,
		projectA,
		ownerA,
	).Scan(&version)

	if err != nil || version != 1 {
		t.Fatalf(
			"expected valid folder with version 1: %d, %v",
			version, err,
		)
	}

	tests := []struct {
		name       string
		tenantID   uuid.UUID
		kind       string
		key        string
		parentID   uuid.UUID
		ownerID    uuid.UUID
		constraint string
		code       string
	}{
		{
			name:       "cross tenant owner",
			tenantID:   tenantA,
			kind:       "project",
			key:        "cross-owner",
			ownerID:    ownerB,
			constraint: "rebac_resources_owner_fk",
			code:       "23503",
		},
		{
			name:       "cross tenant parent",
			tenantID:   tenantB,
			kind:       "folder",
			key:        "cross-parent",
			parentID:   projectA,
			ownerID:    ownerB,
			constraint: "rebac_resources_parent_fk",
			code:       "23503",
		},
		{
			name:       "duplicate resource identity",
			tenantID:   tenantA,
			kind:       "project",
			key:        "website",
			ownerID:    ownerA,
			constraint: "rebac_resources_identity_unique",
			code:       "23505",
		},
		{
			name:       "document without parent",
			tenantID:   tenantA,
			kind:       "document",
			key:        "orphan",
			ownerID:    ownerA,
			constraint: "rebac_resources_parent_shape_check",
			code:       "23514",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				_ = tx.Rollback(context.Background())
			}()

			var parent any
			if tt.parentID != uuid.Nil {
				parent = tt.parentID
			}

			_, err = tx.Exec(
				ctx,
				`INSERT INTO rebac_resources (
                    organization_id,
                    resource_type,
                    external_key,
                    parent_resource_id,
                    owner_membership_id
                )
                VALUES ($1, $2, $3, $4, $5)`,
				tt.tenantID,
				tt.kind,
				tt.key,
				parent,
				tt.ownerID,
			)

			// Deferred foreign keys are checked at COMMIT.
			if err == nil {
				err = tx.Commit(ctx)
			}

			var pgErr *pgconn.PgError

			if !errors.As(err, &pgErr) ||
				pgErr.Code != tt.code ||
				pgErr.ConstraintName != tt.constraint {
				t.Fatalf(
					"expected %s (%s), got %v",
					tt.constraint,
					tt.code,
					err,
				)
			}
		})
	}
}
