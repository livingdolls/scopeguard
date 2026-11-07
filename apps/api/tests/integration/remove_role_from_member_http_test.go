//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestRemoveRoleFromMemberHTTPLifecycle(t *testing.T) {
	f := newAssignmentHTTPFixture(t)

	// First assign a second role to the membership.
	response := f.grant.api.request(
		t, http.MethodPost, f.path, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusCreated)

	// The first DELETE removes the custom role.
	response = f.grant.api.request(
		t, http.MethodDelete, f.path, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusOK)

	var output struct {
		MembershipID uuid.UUID `json:"membership_id"`
		RoleID       uuid.UUID `json:"role_id"`
		Removed      bool      `json:"removed"`
	}
	decodeAuthHTTPJSON(t, response, &output)

	if output.MembershipID != f.membershipID ||
		output.RoleID != f.grant.roleID ||
		!output.Removed {
		t.Fatalf("unexpected removal: %+v", output)
	}

	// An authorized repeated removal is a no-op.
	response = f.grant.api.request(
		t, http.MethodDelete, f.path, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusOK)

	decodeAuthHTTPJSON(t, response, &output)
	if output.Removed {
		t.Fatal("repeated removal must return removed=false")
	}

	var count int

	err := f.grant.api.pool.QueryRow(
		context.Background(),
		`SELECT COUNT(*)
         FROM membership_roles
         WHERE organization_id = $1
           AND membership_id = $2`,
		f.grant.organizationID,
		f.membershipID,
	).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatalf("expected default Member role to remain, got %d", count)
	}
}

func TestRemoveRoleFromMemberHTTPValidation(t *testing.T) {
	f := newAssignmentHTTPFixture(t)

	// Authentication is required.
	response := f.grant.api.request(
		t, http.MethodDelete, f.path, "", "",
	)
	requireAuthHTTPStatus(t, response, http.StatusUnauthorized)

	prefix := "/v1/organizations/" + f.grant.organizationID.String()

	invalidPaths := []string{
		"/v1/organizations/invalid/memberships/" +
			f.membershipID.String() + "/roles/" + f.grant.roleID.String(),
		prefix + "/memberships/invalid/roles/" + f.grant.roleID.String(),
		prefix + "/memberships/" + f.membershipID.String() + "/roles/invalid",
	}

	for _, path := range invalidPaths {
		response = f.grant.api.request(
			t, http.MethodDelete, path, "", f.grant.token,
		)
		requireAuthHTTPStatus(t, response, http.StatusBadRequest)
	}

	// Unknown role.
	response = f.grant.api.request(
		t, http.MethodDelete,
		fmt.Sprintf(
			"%s/memberships/%s/roles/%s",
			prefix,
			f.membershipID,
			uuid.New(),
		),
		"",
		f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusNotFound)

	// Unknown membership.
	response = f.grant.api.request(
		t, http.MethodDelete,
		fmt.Sprintf(
			"%s/memberships/%s/roles/%s",
			prefix,
			uuid.New(),
			f.grant.roleID,
		),
		"",
		f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusNotFound)
}

func TestRemoveRoleFromMemberHTTPRejectsLastRole(t *testing.T) {
	f := newAssignmentHTTPFixture(t)

	// New members initially have exactly one role: member.
	var memberRoleID uuid.UUID

	err := f.grant.api.pool.QueryRow(
		context.Background(),
		`SELECT id
         FROM roles
         WHERE organization_id = $1 AND slug = 'member'`,
		f.grant.organizationID,
	).Scan(&memberRoleID)
	if err != nil {
		t.Fatal(err)
	}

	path := fmt.Sprintf(
		"/v1/organizations/%s/memberships/%s/roles/%s",
		f.grant.organizationID,
		f.membershipID,
		memberRoleID,
	)

	response := f.grant.api.request(
		t, http.MethodDelete, path, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusConflict)

	// Ensure the last role was not deleted.
	var count int
	err = f.grant.api.pool.QueryRow(
		context.Background(),
		`SELECT COUNT(*)
         FROM membership_roles
         WHERE organization_id = $1
           AND membership_id = $2
           AND role_id = $3`,
		f.grant.organizationID,
		f.membershipID,
		memberRoleID,
	).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatal("last membership role was removed")
	}
}

func TestRemoveRoleFromMemberHTTPRejectsOwnerRole(t *testing.T) {
	f := newAssignmentHTTPFixture(t)

	var ownerRoleID uuid.UUID

	err := f.grant.api.pool.QueryRow(
		context.Background(),
		`SELECT id
         FROM roles
         WHERE organization_id = $1 AND slug = 'owner'`,
		f.grant.organizationID,
	).Scan(&ownerRoleID)
	if err != nil {
		t.Fatal(err)
	}

	path := fmt.Sprintf(
		"/v1/organizations/%s/memberships/%s/roles/%s",
		f.grant.organizationID,
		f.membershipID,
		ownerRoleID,
	)

	response := f.grant.api.request(
		t, http.MethodDelete, path, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusConflict)
}

func TestRemoveRoleFromMemberHTTPRequiresPermission(t *testing.T) {
	f := newAssignmentHTTPFixture(t)

	response := f.grant.api.request(
		t, http.MethodPost, f.path, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusCreated)

	// Remove member:role_assign from the actor.
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
		t.Fatal("expected actor permission to be revoked")
	}

	response = f.grant.api.request(
		t, http.MethodDelete, f.path, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusForbidden)

	assertMembershipRoleAssignmentCount(
		t, f, 1,
	)
}

func TestRemoveRoleFromMemberHTTPRejectsPrivilegeEscalation(
	t *testing.T,
) {
	f := newAssignmentHTTPFixture(t)

	// Give Editor member:invite while actor still holds it.
	response := f.grant.api.request(
		t,
		http.MethodPost,
		f.grant.path,
		`{"permission_code":"member:invite"}`,
		f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusCreated)

	// Assign the Editor role before revoking actor's permission.
	response = f.grant.api.request(
		t, http.MethodPost, f.path, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusCreated)

	// Actor loses member:invite but retains member:role_assign.
	result, err := f.grant.api.pool.Exec(
		context.Background(),
		`DELETE FROM role_permissions rp
         USING roles r
         WHERE rp.organization_id = r.organization_id
           AND rp.role_id = r.id
           AND r.organization_id = $1
           AND r.slug = 'owner'
           AND rp.permission_code = 'member:invite'`,
		f.grant.organizationID,
	)
	if err != nil {
		t.Fatal(err)
	}

	if result.RowsAffected() != 1 {
		t.Fatal("expected permission to be revoked")
	}

	// The actor cannot manage a role with a permission
	// they no longer possess.
	response = f.grant.api.request(
		t, http.MethodDelete, f.path, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusForbidden)

	assertMembershipRoleAssignmentCount(t, f, 1)
}

func assertMembershipRoleAssignmentCount(
	t *testing.T,
	f assignmentHTTPFixture,
	want int,
) {
	t.Helper()

	var count int

	err := f.grant.api.pool.QueryRow(
		context.Background(),
		`SELECT COUNT(*)
         FROM membership_roles
         WHERE organization_id = $1
           AND membership_id = $2
           AND role_id = $3`,
		f.grant.organizationID,
		f.membershipID,
		f.grant.roleID,
	).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	if count != want {
		t.Fatalf(
			"expected %d role assignments, got %d",
			want, count,
		)
	}
}

func TestRemoveRoleFromMemberHTTPRejectsCrossTenantTargets(
	t *testing.T,
) {
	f := newAssignmentHTTPFixture(t)

	// Keep a real assignment in Tenant A.
	response := f.grant.api.request(
		t, http.MethodPost, f.path, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusCreated)

	// Create Tenant B owned by the same actor.
	slug := uniqueOrganizationSlug()
	cleanupOrganization(t, f.grant.api.pool, slug)

	response = f.grant.api.request(
		t,
		http.MethodPost,
		"/v1/organizations",
		fmt.Sprintf(`{"name":"Tenant B","slug":%q}`, slug),
		f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusCreated)

	var tenantB struct {
		ID uuid.UUID `json:"id"`
	}
	decodeAuthHTTPJSON(t, response, &tenantB)

	// Create a target membership in Tenant B.
	membershipB := createAssignmentTestMember(
		t, f.grant.api.pool, tenantB.ID,
	)

	// Create a custom role in Tenant B.
	rolePathB := "/v1/organizations/" + tenantB.ID.String() + "/roles"

	response = f.grant.api.request(
		t, http.MethodPost,
		rolePathB,
		`{"name":"Auditor","slug":"auditor"}`,
		f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusCreated)

	var roleB struct {
		ID uuid.UUID `json:"id"`
	}
	decodeAuthHTTPJSON(t, response, &roleB)

	// Assign Tenant B's role through Tenant B's route.
	assignmentB := fmt.Sprintf(
		"/v1/organizations/%s/memberships/%s/roles/%s",
		tenantB.ID,
		membershipB,
		roleB.ID,
	)

	response = f.grant.api.request(
		t, http.MethodPost, assignmentB, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusCreated)

	// Attempt removal with a foreign role ID.
	foreignRolePath := fmt.Sprintf(
		"/v1/organizations/%s/memberships/%s/roles/%s",
		f.grant.organizationID,
		f.membershipID,
		roleB.ID,
	)

	response = f.grant.api.request(
		t, http.MethodDelete, foreignRolePath, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusNotFound)

	// Attempt removal with a foreign membership ID.
	foreignMembershipPath := fmt.Sprintf(
		"/v1/organizations/%s/memberships/%s/roles/%s",
		f.grant.organizationID,
		membershipB,
		f.grant.roleID,
	)

	response = f.grant.api.request(
		t, http.MethodDelete, foreignMembershipPath, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusNotFound)

	// Verify both tenants' assignments remain unchanged.
	assertMembershipRoleAssignmentCount(t, f, 1)

	var countB int
	err := f.grant.api.pool.QueryRow(
		context.Background(),
		`SELECT COUNT(*)
         FROM membership_roles
         WHERE organization_id = $1
           AND membership_id = $2
           AND role_id = $3`,
		tenantB.ID,
		membershipB,
		roleB.ID,
	).Scan(&countB)
	if err != nil {
		t.Fatal(err)
	}

	if countB != 1 {
		t.Fatal("cross-tenant removal changed Tenant B assignment")
	}
}
