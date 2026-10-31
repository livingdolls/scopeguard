//go:build integration

package integration

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	authorizationpostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/adapters/outbound/postgres"
	authorizationapp "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
	authorizationdomain "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
)

type grantPermissionFixture struct {
	pool           *pgxpool.Pool
	ownerID        uuid.UUID
	organizationID uuid.UUID
	editorRoleID   uuid.UUID
}

func setupGrantPermissionFixture(
	t *testing.T,
) grantPermissionFixture {
	t.Helper()

	pool, _, ownerID := setupVerificationUser(t)

	ctx := context.Background()

	// Activate this test user because only active,
	// verified accounts may delegate permissions.
	_, err := pool.Exec(
		ctx,
		`UPDATE users
         SET status = 'active',
             email_verified_at = NOW()
         WHERE id = $1`,
		ownerID,
	)
	if err != nil {
		t.Fatal(err)
	}

	organizationID := createRoleTestOrganization(
		t,
		pool,
		ownerID,
	)

	createRole := authorizationapp.NewCreateRoleService(
		authorizationpostgres.NewRoleRepository(pool),
	)

	role, err := createRole.Execute(
		ctx,
		authorizationapp.CreateRoleInput{
			OrganizationID: organizationID,
			Name:           "Editor",
			Slug:           "editor",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	return grantPermissionFixture{
		pool:           pool,
		ownerID:        ownerID,
		organizationID: organizationID,
		editorRoleID:   role.ID,
	}
}

func (f grantPermissionFixture) service() *authorizationapp.GrantRolePermissionService {
	return authorizationapp.NewGrantRolePermissionService(
		authorizationpostgres.NewRolePermissionRepository(f.pool),
	)
}

func (f grantPermissionFixture) input(
	permissionCode string,
) authorizationapp.GrantRolePermissionInput {
	return authorizationapp.GrantRolePermissionInput{
		ActorUserID:    f.ownerID,
		OrganizationID: f.organizationID,
		RoleID:         f.editorRoleID,
		PermissionCode: permissionCode,
	}
}

func createPermissionGrantTestRole(t *testing.T, ctx context.Context, pool *pgxpool.Pool, organizationID uuid.UUID) uuid.UUID {
	t.Helper()

	service := authorizationapp.NewCreateRoleService(
		authorizationpostgres.NewRoleRepository(pool),
	)
	output, err := service.Execute(ctx, authorizationapp.CreateRoleInput{
		OrganizationID: organizationID,
		Name:           "Auditor",
		Slug:           "auditor",
	})
	if err != nil {
		t.Fatal(err)
	}

	return output.ID
}

func setupPermissionGrant(t *testing.T) (context.Context, *pgxpool.Pool, authorizationapp.GrantRolePermissionInput) {
	t.Helper()

	pool, _, actorID := setupVerificationUser(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	_, err := pool.Exec(ctx,
		`UPDATE users SET status = 'active', email_verified_at = NOW()
		 WHERE id = $1`,
		actorID,
	)
	if err != nil {
		t.Fatal(err)
	}

	organizationID := createRoleTestOrganization(t, pool, actorID)
	roleID := createPermissionGrantTestRole(t, ctx, pool, organizationID)

	return ctx, pool, authorizationapp.GrantRolePermissionInput{
		ActorUserID:    actorID,
		OrganizationID: organizationID,
		RoleID:         roleID,
		PermissionCode: authorizationdomain.PermissionMemberInvite,
	}
}

func TestGrantRolePermissionPersistenceAndIdempotence(t *testing.T) {
	ctx, pool, input := setupPermissionGrant(t)
	service := authorizationapp.NewGrantRolePermissionService(
		authorizationpostgres.NewRolePermissionRepository(pool),
	)

	for _, wantAdded := range []bool{true, false} {
		output, err := service.Execute(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		if output.Added != wantAdded {
			t.Fatalf("expected Added=%t, got %t", wantAdded, output.Added)
		}

		permissions := getRolePermissions(t, pool, input.OrganizationID, "auditor")
		if !slices.Equal(permissions, []string{input.PermissionCode}) {
			t.Fatalf("expected exactly one persisted grant, got %v", permissions)
		}
	}
}

func TestGrantRolePermissionRequiresActorPermissions(t *testing.T) {
	tests := []struct {
		name             string
		revokePermission string
	}{
		{
			name:             "missing role update permission",
			revokePermission: authorizationdomain.PermissionRoleUpdate,
		},
		{
			name:             "missing delegated permission",
			revokePermission: authorizationdomain.PermissionMemberInvite,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, pool, input := setupPermissionGrant(t)
			// Owner membership and permissions in another tenant must not authorize this grant.
			createRoleTestOrganization(t, pool, input.ActorUserID)
			result, err := pool.Exec(ctx,
				`DELETE FROM role_permissions rp
				 USING membership_roles mr, organization_memberships m
				 WHERE rp.organization_id = mr.organization_id
				   AND rp.role_id = mr.role_id
				   AND mr.organization_id = m.organization_id
				   AND mr.membership_id = m.id
				   AND m.organization_id = $1
				   AND m.user_id = $2
				   AND rp.permission_code = $3`,
				input.OrganizationID, input.ActorUserID, tt.revokePermission,
			)
			if err != nil {
				t.Fatal(err)
			}
			if result.RowsAffected() != 1 {
				t.Fatal("expected to revoke one actor permission")
			}

			service := authorizationapp.NewGrantRolePermissionService(
				authorizationpostgres.NewRolePermissionRepository(pool),
			)
			output, err := service.Execute(ctx, input)
			if !errors.Is(err, authorizationapp.ErrPermissionAssignmentForbidden) {
				t.Fatalf("expected forbidden error, got %v", err)
			}
			if output.Added {
				t.Fatal("unauthorized grant must return Added=false")
			}
			if permissions := getRolePermissions(t, pool, input.OrganizationID, "auditor"); len(permissions) != 0 {
				t.Fatalf("unauthorized grant changed target permissions: %v", permissions)
			}
		})
	}
}

func TestGrantRolePermissionRequiresActiveActor(t *testing.T) {
	for _, status := range []string{"pending_verification", "suspended"} {
		t.Run(status, func(t *testing.T) {
			ctx, pool, input := setupPermissionGrant(t)
			_, err := pool.Exec(ctx,
				`UPDATE users
				 SET status = $2::text,
				     email_verified_at = CASE WHEN $2::text = 'pending_verification'
				                              THEN NULL ELSE email_verified_at END
				 WHERE id = $1`,
				input.ActorUserID, status,
			)
			if err != nil {
				t.Fatal(err)
			}

			service := authorizationapp.NewGrantRolePermissionService(
				authorizationpostgres.NewRolePermissionRepository(pool),
			)
			output, err := service.Execute(ctx, input)
			if !errors.Is(err, authorizationapp.ErrPermissionAssignmentForbidden) {
				t.Fatalf("expected forbidden error, got %v", err)
			}
			if output.Added {
				t.Fatal("inactive actor grant must return Added=false")
			}
			if permissions := getRolePermissions(t, pool, input.OrganizationID, "auditor"); len(permissions) != 0 {
				t.Fatalf("inactive actor changed target permissions: %v", permissions)
			}
		})
	}
}

func TestGrantRolePermissionRequiresTenantMembership(t *testing.T) {
	ctx, pool, input := setupPermissionGrant(t)
	// Retain owner permissions in another tenant while removing this membership.
	createRoleTestOrganization(t, pool, input.ActorUserID)
	result, err := pool.Exec(ctx,
		`DELETE FROM organization_memberships
		 WHERE organization_id = $1 AND user_id = $2`,
		input.OrganizationID, input.ActorUserID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.RowsAffected() != 1 {
		t.Fatal("expected to remove one tenant membership")
	}

	service := authorizationapp.NewGrantRolePermissionService(
		authorizationpostgres.NewRolePermissionRepository(pool),
	)
	output, err := service.Execute(ctx, input)
	if !errors.Is(err, authorizationapp.ErrPermissionAssignmentForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
	if output.Added {
		t.Fatal("cross-tenant actor grant must return Added=false")
	}
	if permissions := getRolePermissions(t, pool, input.OrganizationID, "auditor"); len(permissions) != 0 {
		t.Fatalf("cross-tenant actor changed target permissions: %v", permissions)
	}
}

func TestGrantRolePermissionRejectsProtectedTargets(t *testing.T) {
	tests := []struct {
		name          string
		slug          string
		anotherTenant bool
		want          error
	}{
		{
			name: "owner system role",
			slug: authorizationdomain.RoleOwner,
			want: authorizationapp.ErrSystemRoleImmutable,
		},
		{
			name: "member system role",
			slug: authorizationdomain.RoleMember,
			want: authorizationapp.ErrSystemRoleImmutable,
		},
		{
			name:          "custom role in another tenant",
			slug:          "auditor",
			anotherTenant: true,
			want:          authorizationapp.ErrTargetRoleNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, pool, input := setupPermissionGrant(t)
			targetOrganizationID := input.OrganizationID
			if tt.anotherTenant {
				targetOrganizationID = createRoleTestOrganization(t, pool, input.ActorUserID)
				createPermissionGrantTestRole(t, ctx, pool, targetOrganizationID)
			}

			err := pool.QueryRow(ctx,
				`SELECT id FROM roles WHERE organization_id = $1 AND slug = $2`,
				targetOrganizationID, tt.slug,
			).Scan(&input.RoleID)
			if err != nil {
				t.Fatal(err)
			}
			before := getRolePermissions(t, pool, targetOrganizationID, tt.slug)

			service := authorizationapp.NewGrantRolePermissionService(
				authorizationpostgres.NewRolePermissionRepository(pool),
			)
			output, err := service.Execute(ctx, input)
			if !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
			if output.Added {
				t.Fatal("rejected target grant must return Added=false")
			}

			after := getRolePermissions(t, pool, targetOrganizationID, tt.slug)
			if !slices.Equal(before, after) {
				t.Fatalf("rejected grant changed permissions: before=%v, after=%v", before, after)
			}
			if permissions := getRolePermissions(t, pool, input.OrganizationID, "auditor"); len(permissions) != 0 {
				t.Fatalf("rejected grant changed local custom role permissions: %v", permissions)
			}
		})
	}
}

func TestGrantRolePermissionLifecycle(t *testing.T) {
	f := setupGrantPermissionFixture(t)

	ctx := context.Background()
	service := f.service()

	input := f.input(
		authorizationdomain.PermissionMemberInvite,
	)

	first, err := service.Execute(ctx, input)
	if err != nil {
		t.Fatal(err)
	}

	if !first.Added {
		t.Fatal("first grant should add a permission")
	}

	second, err := service.Execute(ctx, input)
	if err != nil {
		t.Fatal(err)
	}

	if second.Added {
		t.Fatal("duplicate grant should be idempotent")
	}

	var count int

	err = f.pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
         FROM role_permissions
         WHERE organization_id = $1
           AND role_id = $2
           AND permission_code = $3`,
		f.organizationID,
		f.editorRoleID,
		input.PermissionCode,
	).Scan(&count)

	if err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatalf("expected one assignment, got %d", count)
	}
}

func TestGrantRolePermissionCannotDelegateMissingRight(
	t *testing.T,
) {
	f := setupGrantPermissionFixture(t)
	ctx := context.Background()

	// Remove this permission from the Owner role
	// inside this isolated test organization.
	_, err := f.pool.Exec(
		ctx,
		`DELETE FROM role_permissions
         WHERE organization_id = $1
           AND permission_code = $2`,
		f.organizationID,
		authorizationdomain.PermissionMemberInvite,
	)
	if err != nil {
		t.Fatal(err)
	}

	input := f.input(
		authorizationdomain.PermissionMemberInvite,
	)

	_, err = f.service().Execute(ctx, input)

	if !errors.Is(
		err,
		authorizationapp.ErrPermissionAssignmentForbidden,
	) {
		t.Fatalf("expected forbidden, got %v", err)
	}

	// Verify the attempted escalation changed nothing.
	var count int

	err = f.pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
         FROM role_permissions
         WHERE organization_id = $1
           AND role_id = $2
           AND permission_code = $3`,
		f.organizationID,
		f.editorRoleID,
		input.PermissionCode,
	).Scan(&count)

	if err != nil {
		t.Fatal(err)
	}

	if count != 0 {
		t.Fatal("unauthorized permission was assigned")
	}
}

func TestGrantRolePermissionRejectsSystemRole(t *testing.T) {
	f := setupGrantPermissionFixture(t)
	ctx := context.Background()

	var ownerRoleID uuid.UUID

	err := f.pool.QueryRow(
		ctx,
		`SELECT id
         FROM roles
         WHERE organization_id = $1
           AND slug = 'owner'`,
		f.organizationID,
	).Scan(&ownerRoleID)

	if err != nil {
		t.Fatal(err)
	}

	input := f.input(
		authorizationdomain.PermissionMemberInvite,
	)
	input.RoleID = ownerRoleID

	_, err = f.service().Execute(ctx, input)

	if !errors.Is(
		err,
		authorizationapp.ErrSystemRoleImmutable,
	) {
		t.Fatalf("expected immutable role error, got %v", err)
	}
}

func TestGrantRolePermissionRejectsCrossTenantRole(t *testing.T) {
	f := setupGrantPermissionFixture(t)
	ctx := context.Background()

	// Same owner, but a different organization.
	tenantB := createRoleTestOrganization(
		t,
		f.pool,
		f.ownerID,
	)

	createRole := authorizationapp.NewCreateRoleService(
		authorizationpostgres.NewRoleRepository(f.pool),
	)

	roleB, err := createRole.Execute(
		ctx,
		authorizationapp.CreateRoleInput{
			OrganizationID: tenantB,
			Name:           "Auditor",
			Slug:           "auditor",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	input := f.input(
		authorizationdomain.PermissionMemberInvite,
	)

	// Deliberately supply a role belonging to Tenant B.
	input.RoleID = roleB.ID

	_, err = f.service().Execute(ctx, input)

	if !errors.Is(
		err,
		authorizationapp.ErrTargetRoleNotFound,
	) {
		t.Fatalf(
			"expected cross-tenant role rejection, got %v",
			err,
		)
	}
}

func TestGrantRolePermissionConcurrent(t *testing.T) {
	f := setupGrantPermissionFixture(t)

	service := f.service()
	input := f.input(
		authorizationdomain.PermissionMemberInvite,
	)

	type result struct {
		added bool
		err   error
	}

	results := make(chan result, 2)

	var wg sync.WaitGroup

	for i := 0; i < 2; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			output, err := service.Execute(
				context.Background(),
				input,
			)

			results <- result{
				added: output.Added,
				err:   err,
			}
		}()
	}

	wg.Wait()
	close(results)

	inserted := 0
	alreadyExists := 0

	for current := range results {
		if current.err != nil {
			t.Fatal(current.err)
		}

		if current.added {
			inserted++
		} else {
			alreadyExists++
		}
	}

	if inserted != 1 || alreadyExists != 1 {
		t.Fatalf(
			"expected one insert and one no-op, got %d/%d",
			inserted,
			alreadyExists,
		)
	}
}
