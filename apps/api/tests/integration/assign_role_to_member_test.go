//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	authorizationpostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/adapters/outbound/postgres"
	authorizationdomain "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"

	authorizationapp "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
)

func createAssignmentTestMember(
	t *testing.T,
	pool *pgxpool.Pool,
	organizationID uuid.UUID,
) uuid.UUID {
	t.Helper()

	ctx := context.Background()
	userID := uuid.New()
	email := "rbac-member-" + uuid.NewString() + "@example.test"

	_, err := pool.Exec(
		ctx,
		`INSERT INTO users (
            id, email, status, email_verified_at
        )
        VALUES ($1, $2, 'active', NOW())`,
		userID,
		email,
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(
			context.Background(), 5*time.Second,
		)
		defer cancel()

		// Remove membership before deleting the user.
		_, err := pool.Exec(
			cleanupCtx,
			`DELETE FROM organization_memberships
             WHERE organization_id = $1 AND user_id = $2`,
			organizationID, userID,
		)
		if err != nil {
			t.Errorf("cleanup target membership: %v", err)
		}

		_, err = pool.Exec(
			cleanupCtx,
			"DELETE FROM users WHERE id = $1",
			userID,
		)
		if err != nil {
			t.Errorf("cleanup target user: %v", err)
		}
	})

	var membershipID uuid.UUID

	err = pool.QueryRow(
		ctx,
		`INSERT INTO organization_memberships (
            organization_id, user_id, role
        )
        VALUES ($1, $2, 'member')
        RETURNING id`,
		organizationID,
		userID,
	).Scan(&membershipID)
	if err != nil {
		t.Fatal(err)
	}

	// Maintain the legacy Member membership's RBAC assignment.
	result, err := pool.Exec(
		ctx,
		`INSERT INTO membership_roles (
            organization_id, membership_id, role_id
        )
        SELECT $1, $2, r.id
        FROM roles r
        WHERE r.organization_id = $1
          AND r.slug = 'member'`,
		organizationID,
		membershipID,
	)
	if err != nil || result.RowsAffected() != 1 {
		t.Fatalf("assign default Member role: %v", err)
	}

	return membershipID
}

func setupRoleAssignment(
	t *testing.T,
) (
	context.Context,
	*pgxpool.Pool,
	authorizationapp.AssignRoleToMemberInput,
) {
	t.Helper()

	ctx, pool, grantInput := setupPermissionGrant(t)

	membershipID := createAssignmentTestMember(
		t,
		pool,
		grantInput.OrganizationID,
	)

	return ctx, pool, authorizationapp.AssignRoleToMemberInput{
		ActorUserID:    grantInput.ActorUserID,
		OrganizationID: grantInput.OrganizationID,
		MembershipID:   membershipID,
		RoleID:         grantInput.RoleID,
	}
}

func TestAssignRoleToMemberLifecycle(t *testing.T) {
	ctx, pool, input := setupRoleAssignment(t)

	service := authorizationapp.NewAssignRoleToMemberService(
		authorizationpostgres.NewMembershipRoleRepository(pool),
	)

	first, err := service.Execute(ctx, input)
	if err != nil || !first.Assigned {
		t.Fatalf("expected assignment: %v", err)
	}

	second, err := service.Execute(ctx, input)
	if err != nil || second.Assigned {
		t.Fatalf("expected idempotent no-op: %v", err)
	}

	var count int

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
         FROM membership_roles
         WHERE organization_id = $1
           AND membership_id = $2
           AND role_id = $3`,
		input.OrganizationID,
		input.MembershipID,
		input.RoleID,
	).Scan(&count)

	if err != nil || count != 1 {
		t.Fatalf("expected one assignment, got %d, %v", count, err)
	}
}

func TestAssignRoleToMemberRequiresPermission(t *testing.T) {
	ctx, pool, input := setupRoleAssignment(t)

	// Remove this permission from the test Owner role.
	_, err := pool.Exec(
		ctx,
		`DELETE FROM role_permissions rp
         USING roles r
         WHERE rp.organization_id = r.organization_id
           AND rp.role_id = r.id
           AND r.organization_id = $1
           AND r.slug = 'owner'
           AND rp.permission_code = $2`,
		input.OrganizationID,
		authorizationdomain.PermissionMemberRoleAssign,
	)
	if err != nil {
		t.Fatal(err)
	}

	service := authorizationapp.NewAssignRoleToMemberService(
		authorizationpostgres.NewMembershipRoleRepository(pool),
	)

	_, err = service.Execute(ctx, input)

	if !errors.Is(
		err,
		authorizationapp.ErrPermissionAssignmentForbidden,
	) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestAssignRoleToMemberRejectsPrivilegeEscalation(t *testing.T) {
	ctx, pool, input := setupRoleAssignment(t)

	permissionCode := authorizationdomain.PermissionMemberInvite

	grantService := authorizationapp.NewGrantRolePermissionService(
		authorizationpostgres.NewRolePermissionRepository(pool),
	)

	_, err := grantService.Execute(
		ctx,
		authorizationapp.GrantRolePermissionInput{
			ActorUserID:    input.ActorUserID,
			OrganizationID: input.OrganizationID,
			RoleID:         input.RoleID,
			PermissionCode: permissionCode,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	// Remove permission from Owner, but keep it in Auditor.
	_, err = pool.Exec(
		ctx,
		`DELETE FROM role_permissions rp
         USING roles r
         WHERE rp.organization_id = r.organization_id
           AND rp.role_id = r.id
           AND r.organization_id = $1
           AND r.slug = 'owner'
           AND rp.permission_code = $2`,
		input.OrganizationID,
		permissionCode,
	)
	if err != nil {
		t.Fatal(err)
	}

	service := authorizationapp.NewAssignRoleToMemberService(
		authorizationpostgres.NewMembershipRoleRepository(pool),
	)

	_, err = service.Execute(ctx, input)

	if !errors.Is(
		err,
		authorizationapp.ErrPermissionAssignmentForbidden,
	) {
		t.Fatalf("expected privilege escalation rejection: %v", err)
	}
}

func TestAssignRoleToMemberRejectsOwnerRole(t *testing.T) {
	ctx, pool, input := setupRoleAssignment(t)

	err := pool.QueryRow(
		ctx,
		`SELECT id FROM roles
         WHERE organization_id = $1
           AND slug = 'owner'`,
		input.OrganizationID,
	).Scan(&input.RoleID)
	if err != nil {
		t.Fatal(err)
	}

	service := authorizationapp.NewAssignRoleToMemberService(
		authorizationpostgres.NewMembershipRoleRepository(pool),
	)

	_, err = service.Execute(ctx, input)

	if !errors.Is(
		err,
		authorizationapp.ErrOwnerRoleAssignmentForbidden,
	) {
		t.Fatalf("expected owner assignment restriction: %v", err)
	}
}

func TestAssignRoleToMemberRejectsCrossTenantRole(t *testing.T) {
	ctx, pool, input := setupRoleAssignment(t)

	tenantB := createRoleTestOrganization(
		t, pool, input.ActorUserID,
	)

	err := pool.QueryRow(
		ctx,
		`SELECT id FROM roles
         WHERE organization_id = $1
           AND slug = 'member'`,
		tenantB,
	).Scan(&input.RoleID)
	if err != nil {
		t.Fatal(err)
	}

	service := authorizationapp.NewAssignRoleToMemberService(
		authorizationpostgres.NewMembershipRoleRepository(pool),
	)

	_, err = service.Execute(ctx, input)

	if !errors.Is(err, authorizationapp.ErrTargetRoleNotFound) {
		t.Fatalf("expected cross-tenant role rejection: %v", err)
	}
}

func TestAssignRoleToMemberRejectsCrossTenantMembership(t *testing.T) {
	ctx, pool, input := setupRoleAssignment(t)

	tenantB := createRoleTestOrganization(
		t, pool, input.ActorUserID,
	)

	err := pool.QueryRow(
		ctx,
		`SELECT id FROM organization_memberships
         WHERE organization_id = $1
           AND user_id = $2`,
		tenantB,
		input.ActorUserID,
	).Scan(&input.MembershipID)
	if err != nil {
		t.Fatal(err)
	}

	service := authorizationapp.NewAssignRoleToMemberService(
		authorizationpostgres.NewMembershipRoleRepository(pool),
	)

	_, err = service.Execute(ctx, input)

	if !errors.Is(err, authorizationapp.ErrMembershipNotFound) {
		t.Fatalf("expected cross-tenant membership rejection: %v", err)
	}
}

func TestAssignRoleToMemberConcurrent(t *testing.T) {
	ctx, pool, input := setupRoleAssignment(t)

	service := authorizationapp.NewAssignRoleToMemberService(
		authorizationpostgres.NewMembershipRoleRepository(pool),
	)

	operation := func(ctx context.Context) (bool, error) {
		output, err := service.Execute(ctx, input)
		return output.Assigned, err
	}

	results := runConcurrentRolePermissionMutations(
		t,
		ctx,
		pool,
		input.OrganizationID,
		operation,
		operation,
	)

	if results[0] == results[1] {
		t.Fatalf(
			"expected one insert and one no-op, got %v",
			results,
		)
	}
}
