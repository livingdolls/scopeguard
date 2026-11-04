//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	authorizationdomain "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"

	authorizationpostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/adapters/outbound/postgres"
	authorizationapp "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
)

func newTestAuthorizationEvaluator(
	pool *pgxpool.Pool,
) authorizationapp.AuthorizationEvaluator {
	return authorizationapp.NewAuthorizationEvaluatorService(
		authorizationpostgres.NewPermissionReader(pool),
	)
}

func membershipUserID(
	t *testing.T,
	pool *pgxpool.Pool,
	organizationID uuid.UUID,
	membershipID uuid.UUID,
) uuid.UUID {
	t.Helper()

	var userID uuid.UUID

	err := pool.QueryRow(
		context.Background(),
		`SELECT user_id
         FROM organization_memberships
         WHERE organization_id = $1
           AND id = $2`,
		organizationID,
		membershipID,
	).Scan(&userID)

	if err != nil {
		t.Fatal(err)
	}

	return userID
}

func TestAuthorizationEvaluatorSystemRoles(t *testing.T) {
	ctx, pool, input := setupRoleAssignment(t)

	evaluator := newTestAuthorizationEvaluator(pool)

	memberUserID := membershipUserID(
		t,
		pool,
		input.OrganizationID,
		input.MembershipID,
	)

	tests := []struct {
		name       string
		userID     uuid.UUID
		permission string
		allowed    bool
	}{
		{
			name:       "owner can manage roles",
			userID:     input.ActorUserID,
			permission: authorizationdomain.PermissionRoleCreate,
			allowed:    true,
		},
		{
			name:       "member can read organization",
			userID:     memberUserID,
			permission: authorizationdomain.PermissionOrganizationRead,
			allowed:    true,
		},
		{
			name:       "member cannot invite",
			userID:     memberUserID,
			permission: authorizationdomain.PermissionMemberInvite,
			allowed:    false,
		},
		{
			name:       "unknown permission is denied",
			userID:     input.ActorUserID,
			permission: "billing:refund",
			allowed:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := evaluator.Can(
				ctx,
				tt.userID,
				input.OrganizationID,
				tt.permission,
			)

			if err != nil {
				t.Fatal(err)
			}

			if allowed != tt.allowed {
				t.Fatalf(
					"expected allowed=%t, got %t",
					tt.allowed,
					allowed,
				)
			}
		})
	}
}

func TestAuthorizationEvaluatorReflectsRBACChanges(
	t *testing.T,
) {
	ctx, pool, input := setupRoleAssignment(t)

	evaluator := newTestAuthorizationEvaluator(pool)

	memberUserID := membershipUserID(
		t, pool, input.OrganizationID, input.MembershipID,
	)

	permission := authorizationdomain.PermissionMemberInvite

	check := func(want bool) {
		t.Helper()

		allowed, err := evaluator.Can(
			ctx,
			memberUserID,
			input.OrganizationID,
			permission,
		)
		if err != nil {
			t.Fatal(err)
		}
		if allowed != want {
			t.Fatalf(
				"expected allowed=%t, got %t",
				want, allowed,
			)
		}
	}

	// A Member does not have member:invite by default.
	check(false)

	rolePermissions := authorizationpostgres.
		NewRolePermissionRepository(pool)

	grant := authorizationapp.NewGrantRolePermissionService(
		rolePermissions,
	)

	permissionInput := authorizationapp.GrantRolePermissionInput{
		ActorUserID:    input.ActorUserID,
		OrganizationID: input.OrganizationID,
		RoleID:         input.RoleID,
		PermissionCode: permission,
	}

	// Give Auditor the permission.
	if _, err := grant.Execute(ctx, permissionInput); err != nil {
		t.Fatal(err)
	}

	// The target Member does not have Auditor yet.
	check(false)

	assign := authorizationapp.NewAssignRoleToMemberService(
		authorizationpostgres.NewMembershipRoleRepository(pool),
	)

	if _, err := assign.Execute(ctx, input); err != nil {
		t.Fatal(err)
	}

	// Effective permissions combine Member + Auditor.
	check(true)

	revoke := authorizationapp.NewRevokeRolePermissionService(
		rolePermissions,
	)

	if _, err := revoke.Execute(
		ctx,
		authorizationapp.RevokeRolePermissionInput(
			permissionInput,
		),
	); err != nil {
		t.Fatal(err)
	}

	// Revoke is reflected without restarting the API.
	check(false)
}

func TestAuthorizationEvaluatorTenantIsolation(t *testing.T) {
	ctx, pool, input := setupRoleAssignment(t)

	evaluator := newTestAuthorizationEvaluator(pool)

	memberUserID := membershipUserID(
		t,
		pool,
		input.OrganizationID,
		input.MembershipID,
	)

	// Owner creates another organization.
	tenantB := createRoleTestOrganization(
		t,
		pool,
		input.ActorUserID,
	)

	// The target Member belongs only to Tenant A.
	allowed, err := evaluator.Can(
		ctx,
		memberUserID,
		tenantB,
		authorizationdomain.PermissionOrganizationRead,
	)

	if err != nil {
		t.Fatal(err)
	}

	if allowed {
		t.Fatal("permission leaked across tenants")
	}

	// Their own Tenant A membership remains valid.
	allowed, err = evaluator.Can(
		ctx,
		memberUserID,
		input.OrganizationID,
		authorizationdomain.PermissionOrganizationRead,
	)

	if err != nil || !allowed {
		t.Fatalf(
			"expected access in own tenant, got %t, %v",
			allowed, err,
		)
	}
}

func TestAuthorizationEvaluatorRejectsSuspendedUser(
	t *testing.T,
) {
	ctx, pool, input := setupRoleAssignment(t)

	evaluator := newTestAuthorizationEvaluator(pool)

	memberUserID := membershipUserID(
		t,
		pool,
		input.OrganizationID,
		input.MembershipID,
	)

	permission := authorizationdomain.PermissionOrganizationRead

	allowed, err := evaluator.Can(
		ctx,
		memberUserID,
		input.OrganizationID,
		permission,
	)
	if err != nil || !allowed {
		t.Fatalf("expected initial access: %v", err)
	}

	_, err = pool.Exec(
		ctx,
		`UPDATE users
         SET status = 'suspended'
         WHERE id = $1`,
		memberUserID,
	)
	if err != nil {
		t.Fatal(err)
	}

	allowed, err = evaluator.Can(
		ctx,
		memberUserID,
		input.OrganizationID,
		permission,
	)

	if err != nil {
		t.Fatal(err)
	}

	if allowed {
		t.Fatal("suspended user must not retain access")
	}
}

func TestAuthorizationEvaluatorRejectsRemovedMembership(
	t *testing.T,
) {
	ctx, pool, input := setupRoleAssignment(t)

	evaluator := newTestAuthorizationEvaluator(pool)

	memberUserID := membershipUserID(
		t,
		pool,
		input.OrganizationID,
		input.MembershipID,
	)

	_, err := pool.Exec(
		ctx,
		`DELETE FROM organization_memberships
         WHERE organization_id = $1
           AND id = $2`,
		input.OrganizationID,
		input.MembershipID,
	)
	if err != nil {
		t.Fatal(err)
	}

	allowed, err := evaluator.Can(
		ctx,
		memberUserID,
		input.OrganizationID,
		authorizationdomain.PermissionOrganizationRead,
	)

	if err != nil {
		t.Fatal(err)
	}

	if allowed {
		t.Fatal("removed membership must not have access")
	}
}
