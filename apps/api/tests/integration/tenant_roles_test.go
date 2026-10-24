//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	organizationpostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/organization/adapters/outbound/postgres"
	organizationapp "github.com/livingdolls/scopeguard/apps/api/internal/modules/organization/application"
)

func createRoleTestOrganization(
	t *testing.T,
	pool *pgxpool.Pool,
	ownerID uuid.UUID,
) uuid.UUID {
	t.Helper()

	service := organizationapp.NewCreateOrganizationService(
		organizationpostgres.NewOrganizationRepository(pool),
	)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	output, err := service.Execute(
		ctx,
		organizationapp.CreateOrganizationInput{
			Name:        "RBAC Test Organization",
			Slug:        "rbac-" + uuid.NewString(),
			OwnerUserID: ownerID,
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		cleanupCtx, done := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer done()

		_, err := pool.Exec(
			cleanupCtx,
			"DELETE FROM organizations WHERE id = $1",
			output.ID,
		)
		if err != nil {
			t.Errorf("cleanup organization: %v", err)
		}
	})

	return output.ID
}

func TestOrganizationCreatesSystemRoles(t *testing.T) {
	pool, _, ownerID := setupVerificationUser(t)

	organizationID := createRoleTestOrganization(
		t, pool, ownerID,
	)

	ctx := context.Background()

	var systemRoleCount int

	err := pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
         FROM roles
         WHERE organization_id = $1
           AND is_system = TRUE
           AND slug IN ('owner', 'member')`,
		organizationID,
	).Scan(&systemRoleCount)

	if err != nil {
		t.Fatal(err)
	}

	if systemRoleCount != 2 {
		t.Fatalf(
			"expected two system roles, got %d",
			systemRoleCount,
		)
	}

	var assignedRole string

	err = pool.QueryRow(
		ctx,
		`SELECT r.slug
         FROM organization_memberships m
         JOIN membership_roles mr
           ON mr.organization_id = m.organization_id
          AND mr.membership_id = m.id
         JOIN roles r
           ON r.organization_id = mr.organization_id
          AND r.id = mr.role_id
         WHERE m.organization_id = $1
           AND m.user_id = $2`,
		organizationID,
		ownerID,
	).Scan(&assignedRole)

	if err != nil {
		t.Fatal(err)
	}

	if assignedRole != "owner" {
		t.Fatalf(
			"expected owner role, got %q",
			assignedRole,
		)
	}
}

func TestRoleSlugUniquePerTenant(t *testing.T) {
	pool, _, ownerID := setupVerificationUser(t)
	tenantA := createRoleTestOrganization(t, pool, ownerID)
	tenantB := createRoleTestOrganization(t, pool, ownerID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const insertRole = `
		INSERT INTO roles (id, organization_id, name, slug, is_system)
		VALUES ($1, $2, 'Auditor', 'auditor', FALSE)
	`
	for _, tenant := range []struct {
		name string
		id   uuid.UUID
	}{
		{name: "Tenant A", id: tenantA},
		{name: "Tenant B", id: tenantB},
	} {
		if _, err := pool.Exec(ctx, insertRole, uuid.New(), tenant.id); err != nil {
			t.Fatalf("insert custom auditor role in %s: %v", tenant.name, err)
		}
	}

	// Use a fresh primary key so the duplicate can only violate the tenant/slug constraint.
	_, err := pool.Exec(ctx, insertRole, uuid.New(), tenantA)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" ||
		pgErr.ConstraintName != "roles_organization_slug_unique" {
		t.Fatalf("expected tenant role slug unique violation (23505), got %v", err)
	}

	var roleCount int
	err = pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM roles
		 WHERE organization_id IN ($1, $2)
		   AND slug = 'auditor' AND is_system = FALSE`,
		tenantA, tenantB,
	).Scan(&roleCount)
	if err != nil {
		t.Fatalf("count custom auditor roles: %v", err)
	}
	if roleCount != 2 {
		t.Fatalf("expected one auditor role per tenant, got %d roles", roleCount)
	}
}

func TestCrossTenantRoleAssignmentRejected(t *testing.T) {
	pool, _, ownerID := setupVerificationUser(t)

	tenantA := createRoleTestOrganization(
		t, pool, ownerID,
	)
	tenantB := createRoleTestOrganization(
		t, pool, ownerID,
	)

	ctx := context.Background()

	var membershipA uuid.UUID
	var roleB uuid.UUID

	err := pool.QueryRow(
		ctx,
		`SELECT id
         FROM organization_memberships
         WHERE organization_id = $1
           AND user_id = $2`,
		tenantA,
		ownerID,
	).Scan(&membershipA)

	if err != nil {
		t.Fatal(err)
	}

	err = pool.QueryRow(
		ctx,
		`SELECT id
         FROM roles
         WHERE organization_id = $1
           AND slug = 'member'`,
		tenantB,
	).Scan(&roleB)

	if err != nil {
		t.Fatal(err)
	}

	_, err = pool.Exec(
		ctx,
		`INSERT INTO membership_roles (
            organization_id,
            membership_id,
            role_id
        ) VALUES ($1, $2, $3)`,
		tenantA,
		membershipA,
		roleB,
	)

	var pgErr *pgconn.PgError

	if !errors.As(err, &pgErr) ||
		pgErr.Code != "23503" {
		t.Fatalf(
			"expected foreign key violation, got %v",
			err,
		)
	}
}
