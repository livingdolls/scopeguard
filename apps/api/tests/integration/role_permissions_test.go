//go:build integration

package integration

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	authorizationpostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/adapters/outbound/postgres"
	authorizationapp "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"

	authorizationdomain "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
)

func getRolePermissions(
	t *testing.T,
	pool *pgxpool.Pool,
	organizationID uuid.UUID,
	roleSlug string,
) []string {
	t.Helper()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	rows, err := pool.Query(
		ctx,
		`SELECT rp.permission_code
         FROM roles r
         JOIN role_permissions rp
           ON rp.organization_id = r.organization_id
          AND rp.role_id = r.id
         WHERE r.organization_id = $1
           AND r.slug = $2
         ORDER BY rp.permission_code`,
		organizationID,
		roleSlug,
	)

	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var permissions []string

	for rows.Next() {
		var code string

		if err := rows.Scan(&code); err != nil {
			t.Fatal(err)
		}

		permissions = append(permissions, code)
	}

	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	return permissions
}

func TestNewOrganizationDefaultPermissions(t *testing.T) {
	pool, _, ownerID := setupVerificationUser(t)

	organizationID := createRoleTestOrganization(
		t, pool, ownerID,
	)

	tests := []struct {
		slug string
	}{
		{slug: authorizationdomain.RoleOwner},
		{slug: authorizationdomain.RoleMember},
	}

	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			expected :=
				authorizationdomain.DefaultSystemRolePermissions(
					tt.slug,
				)

			actual := getRolePermissions(
				t,
				pool,
				organizationID,
				tt.slug,
			)

			slices.Sort(expected)
			slices.Sort(actual)

			if !slices.Equal(actual, expected) {
				t.Fatalf(
					"incorrect %s permissions:\n"+
						"expected: %v\nactual: %v",
					tt.slug,
					expected,
					actual,
				)
			}
		})
	}
}

func TestCustomRoleStartsWithoutPermissions(t *testing.T) {
	pool, _, ownerID := setupVerificationUser(t)

	organizationID := createRoleTestOrganization(
		t, pool, ownerID,
	)

	service := authorizationapp.NewCreateRoleService(
		authorizationpostgres.NewRoleRepository(pool),
	)

	_, err := service.Execute(
		context.Background(),
		authorizationapp.CreateRoleInput{
			OrganizationID: organizationID,
			Name:           "Auditor",
			Slug:           "auditor",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	permissions := getRolePermissions(
		t,
		pool,
		organizationID,
		"auditor",
	)

	if len(permissions) != 0 {
		t.Fatalf(
			"custom role unexpectedly has permissions: %v",
			permissions,
		)
	}
}

func TestCrossTenantRolePermissionRejected(t *testing.T) {
	pool, _, ownerID := setupVerificationUser(t)

	tenantA := createRoleTestOrganization(t, pool, ownerID)
	tenantB := createRoleTestOrganization(t, pool, ownerID)

	ctx := context.Background()

	var roleB uuid.UUID

	err := pool.QueryRow(
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

	// Claim that Tenant B's role belongs to Tenant A.
	_, err = pool.Exec(
		ctx,
		`INSERT INTO role_permissions (
            organization_id,
            role_id,
            permission_code
         )
         VALUES ($1, $2, $3)`,
		tenantA,
		roleB,
		authorizationdomain.PermissionOrganizationRead,
	)

	var pgErr *pgconn.PgError

	if !errors.As(err, &pgErr) ||
		pgErr.Code != "23503" ||
		pgErr.ConstraintName != "role_permissions_role_fk" {
		t.Fatalf(
			"expected cross-tenant FK rejection, got %v",
			err,
		)
	}
}

func TestUnknownRolePermissionRejected(t *testing.T) {
	pool, _, ownerID := setupVerificationUser(t)

	organizationID := createRoleTestOrganization(
		t, pool, ownerID,
	)

	ctx := context.Background()

	var ownerRoleID uuid.UUID

	err := pool.QueryRow(
		ctx,
		`SELECT id
         FROM roles
         WHERE organization_id = $1
           AND slug = 'owner'`,
		organizationID,
	).Scan(&ownerRoleID)

	if err != nil {
		t.Fatal(err)
	}

	_, err = pool.Exec(
		ctx,
		`INSERT INTO role_permissions (
            organization_id,
            role_id,
            permission_code
        )
        VALUES ($1, $2, $3)`,
		organizationID,
		ownerRoleID,
		"unknown:permission",
	)

	var pgErr *pgconn.PgError

	if !errors.As(err, &pgErr) ||
		pgErr.Code != "23503" ||
		pgErr.ConstraintName != "role_permissions_permission_fk" {
		t.Fatalf(
			"expected unknown permission rejection, got %v",
			err,
		)
	}
}
