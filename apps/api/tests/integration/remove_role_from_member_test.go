//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"

	authorizationpostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/adapters/outbound/postgres"
	authorizationapp "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
)

func TestRemoveRoleFromMemberLifecycle(t *testing.T) {
	ctx, pool, input := setupRoleAssignment(t)

	repository := authorizationpostgres.NewMembershipRoleRepository(pool)

	assign := authorizationapp.NewAssignRoleToMemberService(
		repository,
	)

	remove := authorizationapp.NewRemoveRoleFromMemberService(
		repository,
	)

	assigned, err := assign.Execute(ctx, input)
	if err != nil || !assigned.Assigned {
		t.Fatalf("expected assignment: %v", err)
	}

	removeInput := authorizationapp.RemoveRoleFromMemberInput{
		ActorUserID:    input.ActorUserID,
		OrganizationID: input.OrganizationID,
		MembershipID:   input.MembershipID,
		RoleID:         input.RoleID,
	}

	first, err := remove.Execute(ctx, removeInput)
	if err != nil || !first.Removed {
		t.Fatalf("expected role removal: %v", err)
	}

	second, err := remove.Execute(ctx, removeInput)
	if err != nil || second.Removed {
		t.Fatalf("expected idempotent removal: %v", err)
	}

	var count int

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
         FROM membership_roles
         WHERE organization_id = $1
           AND membership_id = $2`,
		input.OrganizationID,
		input.MembershipID,
	).Scan(&count)

	if err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatalf(
			"expected remaining Member role, got %d roles",
			count,
		)
	}
}

func TestRemoveRoleFromMemberRejectsLastRole(t *testing.T) {
	ctx, pool, input := setupRoleAssignment(t)

	// Target initially has exactly one role: Member.
	err := pool.QueryRow(
		ctx,
		`SELECT id FROM roles
         WHERE organization_id = $1
           AND slug = 'member'`,
		input.OrganizationID,
	).Scan(&input.RoleID)

	if err != nil {
		t.Fatal(err)
	}

	service := authorizationapp.NewRemoveRoleFromMemberService(
		authorizationpostgres.NewMembershipRoleRepository(pool),
	)

	_, err = service.Execute(
		ctx,
		authorizationapp.RemoveRoleFromMemberInput(input),
	)

	if !errors.Is(
		err,
		authorizationapp.ErrLastMembershipRole,
	) {
		t.Fatalf("expected last-role protection, got %v", err)
	}

	var count int

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
         FROM membership_roles
         WHERE organization_id = $1
           AND membership_id = $2`,
		input.OrganizationID,
		input.MembershipID,
	).Scan(&count)

	if err != nil || count != 1 {
		t.Fatalf(
			"last role must remain assigned: count=%d, err=%v",
			count, err,
		)
	}
}

func TestRemoveRoleFromMemberRejectsOwner(t *testing.T) {
	ctx, pool, input := setupRoleAssignment(t)

	err := pool.QueryRow(
		ctx,
		`SELECT id
         FROM roles
         WHERE organization_id = $1
           AND slug = 'owner'`,
		input.OrganizationID,
	).Scan(&input.RoleID)
	if err != nil {
		t.Fatal(err)
	}

	// Target the actor's own Owner membership.
	err = pool.QueryRow(
		ctx,
		`SELECT id
         FROM organization_memberships
         WHERE organization_id = $1
           AND user_id = $2`,
		input.OrganizationID,
		input.ActorUserID,
	).Scan(&input.MembershipID)
	if err != nil {
		t.Fatal(err)
	}

	service := authorizationapp.NewRemoveRoleFromMemberService(
		authorizationpostgres.NewMembershipRoleRepository(pool),
	)

	_, err = service.Execute(
		ctx,
		authorizationapp.RemoveRoleFromMemberInput(input),
	)

	if !errors.Is(
		err,
		authorizationapp.ErrOwnerRoleRemovalForbidden,
	) {
		t.Fatalf("expected owner protection, got %v", err)
	}
}

func TestRemoveRoleFromMemberRejectsCrossTenantRole(
	t *testing.T,
) {
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

	service := authorizationapp.NewRemoveRoleFromMemberService(
		authorizationpostgres.NewMembershipRoleRepository(pool),
	)

	_, err = service.Execute(
		ctx,
		authorizationapp.RemoveRoleFromMemberInput(input),
	)

	if !errors.Is(
		err,
		authorizationapp.ErrTargetRoleNotFound,
	) {
		t.Fatalf("expected cross-tenant role rejection: %v", err)
	}
}

func TestRemoveRoleFromMemberRejectsCrossTenantMembership(
	t *testing.T,
) {
	ctx, pool, input := setupRoleAssignment(t)

	tenantB := createRoleTestOrganization(
		t, pool, input.ActorUserID,
	)

	err := pool.QueryRow(
		ctx,
		`SELECT id
         FROM organization_memberships
         WHERE organization_id = $1
           AND user_id = $2`,
		tenantB,
		input.ActorUserID,
	).Scan(&input.MembershipID)
	if err != nil {
		t.Fatal(err)
	}

	service := authorizationapp.NewRemoveRoleFromMemberService(
		authorizationpostgres.NewMembershipRoleRepository(pool),
	)

	_, err = service.Execute(
		ctx,
		authorizationapp.RemoveRoleFromMemberInput(input),
	)

	if !errors.Is(
		err,
		authorizationapp.ErrMembershipNotFound,
	) {
		t.Fatalf(
			"expected cross-tenant membership rejection: %v",
			err,
		)
	}
}

func TestRemoveRoleFromMemberConcurrent(t *testing.T) {
	ctx, pool, input := setupRoleAssignment(t)

	repository := authorizationpostgres.NewMembershipRoleRepository(pool)

	assign := authorizationapp.NewAssignRoleToMemberService(
		repository,
	)

	// Give the target a second role.
	output, err := assign.Execute(ctx, input)
	if err != nil || !output.Assigned {
		t.Fatalf("prepare role assignment: %v", err)
	}

	remove := authorizationapp.NewRemoveRoleFromMemberService(
		repository,
	)

	removeInput := authorizationapp.RemoveRoleFromMemberInput(
		input,
	)

	operation := func(ctx context.Context) (bool, error) {
		output, err := remove.Execute(ctx, removeInput)
		return output.Removed, err
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
			"expected one removal and one no-op, got %v",
			results,
		)
	}

	var count int

	err = pool.QueryRow(
		context.Background(),
		`SELECT COUNT(*)
         FROM membership_roles
         WHERE organization_id = $1
           AND membership_id = $2`,
		input.OrganizationID,
		input.MembershipID,
	).Scan(&count)

	if err != nil || count != 1 {
		t.Fatalf(
			"expected one remaining role, count=%d err=%v",
			count, err,
		)
	}
}
