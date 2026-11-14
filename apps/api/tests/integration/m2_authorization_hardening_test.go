//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	authorizationpostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/adapters/outbound/postgres"
	authorizationapp "github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
)

func TestM2ConcurrentDifferentRoleRemovalsPreserveLastRole(
	t *testing.T,
) {
	ctx, pool, input := setupRoleAssignment(t)

	repository := authorizationpostgres.NewMembershipRoleRepository(pool)

	assign := authorizationapp.NewAssignRoleToMemberService(
		repository,
	)
	remove := authorizationapp.NewRemoveRoleFromMemberService(
		repository,
	)

	// Initial roles: member + a custom role.
	assigned, err := assign.Execute(ctx, input)
	if err != nil || !assigned.Assigned {
		t.Fatalf("prepare second role: %+v, %v", assigned, err)
	}

	var memberRoleID uuid.UUID
	err = pool.QueryRow(
		ctx,
		`SELECT id FROM roles
         WHERE organization_id = $1 AND slug = 'member'`,
		input.OrganizationID,
	).Scan(&memberRoleID)
	if err != nil {
		t.Fatal(err)
	}

	var lastRoleConflicts atomic.Int32

	operation := func(roleID uuid.UUID) func(context.Context) (bool, error) {
		return func(opCtx context.Context) (bool, error) {
			output, err := remove.Execute(
				opCtx,
				authorizationapp.RemoveRoleFromMemberInput{
					ActorUserID:    input.ActorUserID,
					OrganizationID: input.OrganizationID,
					MembershipID:   input.MembershipID,
					RoleID:         roleID,
				},
			)

			if errors.Is(err, authorizationapp.ErrLastMembershipRole) {
				lastRoleConflicts.Add(1)
				return false, nil
			}

			return output.Removed, err
		}
	}

	// The helper verifies that both operations wait behind
	// the tenant's FOR UPDATE lock before releasing them.
	changed := runConcurrentRolePermissionMutations(
		t,
		ctx,
		pool,
		input.OrganizationID,
		operation(memberRoleID),
		operation(input.RoleID),
	)

	if changed[0] == changed[1] {
		t.Fatalf(
			"expected one successful removal and one rejection: %v",
			changed,
		)
	}

	if got := lastRoleConflicts.Load(); got != 1 {
		t.Fatalf(
			"expected exactly one ErrLastMembershipRole, got %d",
			got,
		)
	}

	var remaining int

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
         FROM membership_roles
         WHERE organization_id = $1
           AND membership_id = $2`,
		input.OrganizationID,
		input.MembershipID,
	).Scan(&remaining)
	if err != nil {
		t.Fatal(err)
	}

	if remaining != 1 {
		t.Fatalf(
			"membership must retain exactly one role, got %d",
			remaining,
		)
	}
}

func TestM2RoleReadDoesNotRequirePermissionRead(t *testing.T) {
	f := newGrantRoleHTTPFixture(t)

	// Remove permission:read from the actor, keeping role:read.
	result, err := f.api.pool.Exec(
		context.Background(),
		`DELETE FROM role_permissions rp
         USING roles r
         WHERE rp.organization_id = r.organization_id
           AND rp.role_id = r.id
           AND r.organization_id = $1
           AND r.slug = 'owner'
           AND rp.permission_code = 'permission:read'`,
		f.organizationID,
	)
	if err != nil {
		t.Fatal(err)
	}

	if result.RowsAffected() != 1 {
		t.Fatal("expected permission:read to be revoked")
	}

	// Role permission listing must remain available.
	response := f.api.request(
		t,
		http.MethodGet,
		f.path,
		"",
		f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusOK)

	// Permission catalog remains protected separately.
	catalogPath := fmt.Sprintf(
		"/v1/organizations/%s/permissions",
		f.organizationID,
	)

	response = f.api.request(
		t, http.MethodGet, catalogPath, "", f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusForbidden)
}

func TestM2MemberReadDoesNotRequireRoleAssign(t *testing.T) {
	f := newAssignmentHTTPFixture(t)

	// Remove mutation permission while retaining member:read.
	result, err := f.grant.api.pool.Exec(
		context.Background(),
		`DELETE FROM role_permissions rp
         USING roles r
         WHERE rp.organization_id = r.organization_id
           AND rp.role_id = r.id
           AND r.organization_id = $1
           AND r.slug = 'owner'
           AND rp.permission_code = 'member:role_assign'`,
		f.grant.organizationID,
	)
	if err != nil {
		t.Fatal(err)
	}

	if result.RowsAffected() != 1 {
		t.Fatal("expected member:role_assign to be revoked")
	}

	// Read-only access is still allowed.
	response := f.grant.api.request(
		t,
		http.MethodGet,
		membershipRolesListPath(f),
		"",
		f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusOK)

	// Assignment mutation must now be forbidden.
	response = f.grant.api.request(
		t,
		http.MethodPost,
		f.path,
		"",
		f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusForbidden)
}

func TestM2CompositeFKRejectsCrossTenantAssignments(
	t *testing.T,
) {
	ctx, pool, input := setupRoleAssignment(t)

	// Actor also owns another tenant.
	tenantB := createRoleTestOrganization(
		t, pool, input.ActorUserID,
	)
	membershipB := createAssignmentTestMember(
		t, pool, tenantB,
	)

	var roleB uuid.UUID
	err := pool.QueryRow(
		ctx,
		`SELECT id FROM roles
         WHERE organization_id = $1 AND slug = 'member'`,
		tenantB,
	).Scan(&roleB)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		organization uuid.UUID
		membership   uuid.UUID
		role         uuid.UUID
	}{
		{
			name:         "membership from another tenant",
			organization: input.OrganizationID,
			membership:   membershipB,
			role:         input.RoleID,
		},
		{
			name:         "role from another tenant",
			organization: input.OrganizationID,
			membership:   input.MembershipID,
			role:         roleB,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}

			_, err = tx.Exec(
				ctx,
				`INSERT INTO membership_roles (
                    organization_id,
                    membership_id,
                    role_id
                 ) VALUES ($1, $2, $3)`,
				tc.organization,
				tc.membership,
				tc.role,
			)

			// One of the FK constraints is deferred.
			// Therefore an INSERT may succeed but COMMIT must fail.
			if err == nil {
				err = tx.Commit(ctx)
			}

			_ = tx.Rollback(ctx)

			if err == nil {
				t.Fatal("cross-tenant assignment unexpectedly succeeded")
			}

			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
				t.Fatalf(
					"expected foreign-key violation (23503), got %v",
					err,
				)
			}
		})
	}
}
