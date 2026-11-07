//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

type assignmentHTTPFixture struct {
	grant        grantRoleHTTPFixture
	membershipID uuid.UUID
	path         string
}

func newAssignmentHTTPFixture(t *testing.T) assignmentHTTPFixture {
	t.Helper()

	f := newGrantRoleHTTPFixture(t)

	membershipID := createAssignmentTestMember(
		t,
		f.api.pool,
		f.organizationID,
	)

	path := fmt.Sprintf(
		"/v1/organizations/%s/memberships/%s/roles/%s",
		f.organizationID,
		membershipID,
		f.roleID,
	)

	return assignmentHTTPFixture{
		grant:        f,
		membershipID: membershipID,
		path:         path,
	}
}

func TestAssignRoleToMemberHTTPLifecycle(t *testing.T) {
	f := newAssignmentHTTPFixture(t)

	response := f.grant.api.request(
		t, http.MethodPost, f.path, "", "",
	)
	requireAuthHTTPStatus(t, response, http.StatusUnauthorized)

	response = f.grant.api.request(
		t, http.MethodPost, f.path, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusCreated)

	var output struct {
		MembershipID uuid.UUID `json:"membership_id"`
		RoleID       uuid.UUID `json:"role_id"`
		Assigned     bool      `json:"assigned"`
	}
	decodeAuthHTTPJSON(t, response, &output)

	if output.MembershipID != f.membershipID ||
		output.RoleID != f.grant.roleID ||
		!output.Assigned {
		t.Fatalf("unexpected assignment: %+v", output)
	}

	// A repeated authorized request is idempotent.
	response = f.grant.api.request(
		t, http.MethodPost, f.path, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusOK)

	decodeAuthHTTPJSON(t, response, &output)
	if output.Assigned {
		t.Fatal("duplicate role assignment must return false")
	}

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

	if count != 1 {
		t.Fatalf("expected one assignment, got %d", count)
	}
}

func TestAssignRoleToMemberHTTPValidatesIdentifiers(t *testing.T) {
	f := newAssignmentHTTPFixture(t)

	prefix := "/v1/organizations/" + f.grant.organizationID.String()

	paths := []string{
		"/v1/organizations/invalid/memberships/" +
			f.membershipID.String() + "/roles/" + f.grant.roleID.String(),
		prefix + "/memberships/invalid/roles/" + f.grant.roleID.String(),
		prefix + "/memberships/" + f.membershipID.String() + "/roles/invalid",
	}

	for _, path := range paths {
		response := f.grant.api.request(
			t, http.MethodPost, path, "", f.grant.token,
		)
		requireAuthHTTPStatus(t, response, http.StatusBadRequest)
	}

	unknownRole := fmt.Sprintf(
		"/v1/organizations/%s/memberships/%s/roles/%s",
		f.grant.organizationID,
		f.membershipID,
		uuid.New(),
	)

	response := f.grant.api.request(
		t, http.MethodPost, unknownRole, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusNotFound)

	unknownMembership := fmt.Sprintf(
		"/v1/organizations/%s/memberships/%s/roles/%s",
		f.grant.organizationID,
		uuid.New(),
		f.grant.roleID,
	)

	response = f.grant.api.request(
		t, http.MethodPost, unknownMembership, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusNotFound)
}

func TestAssignRoleToMemberHTTPRejectsOwnerRole(t *testing.T) {
	f := newAssignmentHTTPFixture(t)

	var ownerRoleID uuid.UUID
	err := f.grant.api.pool.QueryRow(
		context.Background(),
		`SELECT id FROM roles
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
		t, http.MethodPost, path, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusConflict)
}

func TestAssignRoleToMemberHTTPRequiresPermission(t *testing.T) {
	f := newAssignmentHTTPFixture(t)

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

	response := f.grant.api.request(
		t, http.MethodPost, f.path, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusForbidden)
}

func TestAssignRoleToMemberHTTPRejectsPrivilegeEscalation(
	t *testing.T,
) {
	f := newAssignmentHTTPFixture(t)

	// Give Editor a permission that the actor initially holds.
	response := f.grant.api.request(
		t,
		http.MethodPost,
		f.grant.path,
		`{"permission_code":"member:invite"}`,
		f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusCreated)

	// Revoke that permission from the actor's Owner role.
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
		t.Fatal("expected member:invite to be revoked")
	}

	// Actor still has member:role_assign, but cannot
	// delegate an Editor role containing member:invite.
	response = f.grant.api.request(
		t, http.MethodPost, f.path, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusForbidden)

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
		f.grant.roleID,
	).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("unauthorized assignment was persisted")
	}
}

func TestAssignRoleToMemberHTTPRejectsCrossTenantTargets(
	t *testing.T,
) {
	f := newAssignmentHTTPFixture(t)

	slug := uniqueOrganizationSlug()
	cleanupOrganization(t, f.grant.api.pool, slug)

	// The actor owns Tenant A and Tenant B.
	response := f.grant.api.request(
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

	var roleB, membershipB uuid.UUID

	err := f.grant.api.pool.QueryRow(
		context.Background(),
		`SELECT id FROM roles
         WHERE organization_id = $1 AND slug = 'member'`,
		tenantB.ID,
	).Scan(&roleB)
	if err != nil {
		t.Fatal(err)
	}

	err = f.grant.api.pool.QueryRow(
		context.Background(),
		`SELECT id FROM organization_memberships
         WHERE organization_id = $1`,
		tenantB.ID,
	).Scan(&membershipB)
	if err != nil {
		t.Fatal(err)
	}

	// Tenant B role cannot be assigned through Tenant A.
	foreignRolePath := fmt.Sprintf(
		"/v1/organizations/%s/memberships/%s/roles/%s",
		f.grant.organizationID,
		f.membershipID,
		roleB,
	)

	response = f.grant.api.request(
		t, http.MethodPost, foreignRolePath, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusNotFound)

	// Tenant B membership cannot be changed through Tenant A.
	foreignMembershipPath := fmt.Sprintf(
		"/v1/organizations/%s/memberships/%s/roles/%s",
		f.grant.organizationID,
		membershipB,
		f.grant.roleID,
	)

	response = f.grant.api.request(
		t, http.MethodPost, foreignMembershipPath, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusNotFound)

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
		f.grant.roleID,
	).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("cross-tenant attempt created an assignment")
	}
}
