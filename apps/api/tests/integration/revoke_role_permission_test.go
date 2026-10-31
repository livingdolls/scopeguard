//go:build integration

package integration

import (
	"context"
	"errors"
	"slices"
	"testing"

	authorizationpostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/adapters/outbound/postgres"
	authorizationapp "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
	authorizationdomain "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
)

func TestRevokeRolePermissionLifecycle(t *testing.T) {
	ctx, pool, grantInput := setupPermissionGrant(t)

	repository := authorizationpostgres.NewRolePermissionRepository(
		pool,
	)

	grant := authorizationapp.NewGrantRolePermissionService(
		repository,
	)

	revoke := authorizationapp.NewRevokeRolePermissionService(
		repository,
	)

	// 1. Grant permission.
	granted, err := grant.Execute(ctx, grantInput)
	if err != nil {
		t.Fatal(err)
	}
	if !granted.Added {
		t.Fatal("expected permission to be granted")
	}

	revokeInput := authorizationapp.RevokeRolePermissionInput{
		ActorUserID:    grantInput.ActorUserID,
		OrganizationID: grantInput.OrganizationID,
		RoleID:         grantInput.RoleID,
		PermissionCode: grantInput.PermissionCode,
	}

	// 2. First revoke removes the assignment.
	first, err := revoke.Execute(ctx, revokeInput)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Removed {
		t.Fatal("expected first revoke to remove permission")
	}

	// 3. Second revoke is an authorized no-op.
	second, err := revoke.Execute(ctx, revokeInput)
	if err != nil {
		t.Fatal(err)
	}
	if second.Removed {
		t.Fatal("repeated revoke must be idempotent")
	}

	// 4. Verify database state.
	var count int

	err = pool.QueryRow(
		context.Background(),
		`SELECT COUNT(*)
         FROM role_permissions
         WHERE organization_id = $1
           AND role_id = $2
           AND permission_code = $3`,
		revokeInput.OrganizationID,
		revokeInput.RoleID,
		revokeInput.PermissionCode,
	).Scan(&count)

	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("revoked permission remains in database")
	}
}

func TestRevokeRolePermissionRequiresActorPermissions(t *testing.T) {
	tests := []struct {
		name             string
		revokePermission string
	}{
		{
			name:             "missing role update permission",
			revokePermission: authorizationdomain.PermissionRoleUpdate,
		},
		{
			name:             "missing revoked permission",
			revokePermission: authorizationdomain.PermissionMemberInvite,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, pool, grantInput := setupPermissionGrant(t)
			repository := authorizationpostgres.NewRolePermissionRepository(pool)
			grant := authorizationapp.NewGrantRolePermissionService(repository)
			if output, err := grant.Execute(ctx, grantInput); err != nil || !output.Added {
				t.Fatalf("expected initial grant, got %+v, %v", output, err)
			}

			// Retain owner permissions elsewhere, as in the corresponding Grant test.
			createRoleTestOrganization(t, pool, grantInput.ActorUserID)
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
				grantInput.OrganizationID, grantInput.ActorUserID, tt.revokePermission,
			)
			if err != nil {
				t.Fatal(err)
			}
			if result.RowsAffected() != 1 {
				t.Fatal("expected to revoke one actor permission")
			}

			revoke := authorizationapp.NewRevokeRolePermissionService(repository)
			output, err := revoke.Execute(ctx, authorizationapp.RevokeRolePermissionInput(grantInput))
			if !errors.Is(err, authorizationapp.ErrPermissionAssignmentForbidden) {
				t.Fatalf("expected forbidden error, got %v", err)
			}
			if output.Removed {
				t.Fatal("unauthorized revoke must return Removed=false")
			}
			permissions := getRolePermissions(t, pool, grantInput.OrganizationID, "auditor")
			if !slices.Equal(permissions, []string{grantInput.PermissionCode}) {
				t.Fatalf("unauthorized revoke changed target permissions: %v", permissions)
			}
		})
	}
}

func TestRevokeRolePermissionRequiresActiveActor(t *testing.T) {
	ctx, pool, grantInput := setupPermissionGrant(t)
	repository := authorizationpostgres.NewRolePermissionRepository(pool)
	grant := authorizationapp.NewGrantRolePermissionService(repository)
	if output, err := grant.Execute(ctx, grantInput); err != nil || !output.Added {
		t.Fatalf("expected initial grant, got %+v, %v", output, err)
	}

	_, err := pool.Exec(ctx, `UPDATE users SET status = 'suspended' WHERE id = $1`, grantInput.ActorUserID)
	if err != nil {
		t.Fatal(err)
	}

	revoke := authorizationapp.NewRevokeRolePermissionService(repository)
	output, err := revoke.Execute(ctx, authorizationapp.RevokeRolePermissionInput(grantInput))
	if !errors.Is(err, authorizationapp.ErrPermissionAssignmentForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
	if output.Removed {
		t.Fatal("suspended actor revoke must return Removed=false")
	}
	permissions := getRolePermissions(t, pool, grantInput.OrganizationID, "auditor")
	if !slices.Equal(permissions, []string{grantInput.PermissionCode}) {
		t.Fatalf("suspended actor changed target permissions: %v", permissions)
	}
}

func TestRevokeRolePermissionRequiresTenantMembership(t *testing.T) {
	ctx, pool, grantInput := setupPermissionGrant(t)
	repository := authorizationpostgres.NewRolePermissionRepository(pool)
	grant := authorizationapp.NewGrantRolePermissionService(repository)
	if output, err := grant.Execute(ctx, grantInput); err != nil || !output.Added {
		t.Fatalf("expected initial grant, got %+v, %v", output, err)
	}

	createRoleTestOrganization(t, pool, grantInput.ActorUserID)
	result, err := pool.Exec(ctx,
		`DELETE FROM organization_memberships
		 WHERE organization_id = $1 AND user_id = $2`,
		grantInput.OrganizationID, grantInput.ActorUserID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.RowsAffected() != 1 {
		t.Fatal("expected to remove one tenant membership")
	}

	revoke := authorizationapp.NewRevokeRolePermissionService(repository)
	output, err := revoke.Execute(ctx, authorizationapp.RevokeRolePermissionInput(grantInput))
	if !errors.Is(err, authorizationapp.ErrPermissionAssignmentForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
	if output.Removed {
		t.Fatal("cross-tenant actor revoke must return Removed=false")
	}
	permissions := getRolePermissions(t, pool, grantInput.OrganizationID, "auditor")
	if !slices.Equal(permissions, []string{grantInput.PermissionCode}) {
		t.Fatalf("cross-tenant actor changed target permissions: %v", permissions)
	}
}

func TestRevokeRolePermissionRejectsProtectedTargets(t *testing.T) {
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
			ctx, pool, grantInput := setupPermissionGrant(t)
			repository := authorizationpostgres.NewRolePermissionRepository(pool)
			targetOrganizationID := grantInput.OrganizationID
			input := authorizationapp.RevokeRolePermissionInput(grantInput)
			// Both system roles already hold this permission, so a buggy revoke would delete it.
			input.PermissionCode = authorizationdomain.PermissionOrganizationRead
			if tt.anotherTenant {
				targetOrganizationID = createRoleTestOrganization(t, pool, grantInput.ActorUserID)
				grantInput.OrganizationID = targetOrganizationID
				grantInput.RoleID = createPermissionGrantTestRole(t, ctx, pool, targetOrganizationID)
				grant := authorizationapp.NewGrantRolePermissionService(repository)
				if output, err := grant.Execute(ctx, grantInput); err != nil || !output.Added {
					t.Fatalf("expected initial grant in other tenant, got %+v, %v", output, err)
				}
				input.PermissionCode = grantInput.PermissionCode
			}

			err := pool.QueryRow(ctx,
				`SELECT id FROM roles WHERE organization_id = $1 AND slug = $2`,
				targetOrganizationID, tt.slug,
			).Scan(&input.RoleID)
			if err != nil {
				t.Fatal(err)
			}
			before := getRolePermissions(t, pool, targetOrganizationID, tt.slug)

			revoke := authorizationapp.NewRevokeRolePermissionService(repository)
			output, err := revoke.Execute(ctx, input)
			if !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
			if output.Removed {
				t.Fatal("rejected target revoke must return Removed=false")
			}
			after := getRolePermissions(t, pool, targetOrganizationID, tt.slug)
			if !slices.Equal(before, after) {
				t.Fatalf("rejected revoke changed permissions: before=%v, after=%v", before, after)
			}
			if permissions := getRolePermissions(t, pool, input.OrganizationID, "auditor"); len(permissions) != 0 {
				t.Fatalf("rejected revoke changed local custom role permissions: %v", permissions)
			}
		})
	}
}

func TestRevokeRolePermissionRejectsUnknownPermission(t *testing.T) {
	ctx, pool, grantInput := setupPermissionGrant(t)
	repository := authorizationpostgres.NewRolePermissionRepository(pool)
	grant := authorizationapp.NewGrantRolePermissionService(repository)
	if output, err := grant.Execute(ctx, grantInput); err != nil || !output.Added {
		t.Fatalf("expected initial grant, got %+v, %v", output, err)
	}

	input := authorizationapp.RevokeRolePermissionInput(grantInput)
	input.PermissionCode = "unknown:permission"
	revoke := authorizationapp.NewRevokeRolePermissionService(repository)
	output, err := revoke.Execute(ctx, input)
	if !errors.Is(err, authorizationapp.ErrPermissionNotFound) {
		t.Fatalf("expected permission not found, got %v", err)
	}
	if output.Removed {
		t.Fatal("unknown permission revoke must return Removed=false")
	}
	permissions := getRolePermissions(t, pool, grantInput.OrganizationID, "auditor")
	if !slices.Equal(permissions, []string{grantInput.PermissionCode}) {
		t.Fatalf("unknown permission revoke changed target permissions: %v", permissions)
	}
}
