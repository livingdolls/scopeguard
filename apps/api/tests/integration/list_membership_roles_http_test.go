//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

type membershipRolesHTTPResult struct {
	MembershipID uuid.UUID `json:"membership_id"`
	Roles        []struct {
		ID             uuid.UUID `json:"id"`
		OrganizationID uuid.UUID `json:"organization_id"`
		Slug           string    `json:"slug"`
		IsSystem       bool      `json:"is_system"`
	} `json:"roles"`
}

func membershipRolesListPath(f assignmentHTTPFixture) string {
	return fmt.Sprintf(
		"/v1/organizations/%s/memberships/%s/roles",
		f.grant.organizationID,
		f.membershipID,
	)
}

func TestListMembershipRolesHTTPLifecycle(t *testing.T) {
	f := newAssignmentHTTPFixture(t)
	path := membershipRolesListPath(f)

	// A new member initially has the default member role.
	response := f.grant.api.request(
		t, http.MethodGet, path, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusOK)

	var output membershipRolesHTTPResult
	decodeAuthHTTPJSON(t, response, &output)

	if output.MembershipID != f.membershipID ||
		len(output.Roles) != 1 ||
		output.Roles[0].Slug != "member" ||
		!output.Roles[0].IsSystem {
		t.Fatalf("unexpected initial roles: %+v", output)
	}

	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("expected Cache-Control: no-store")
	}

	// Assign the custom Editor role.
	response = f.grant.api.request(
		t, http.MethodPost, f.path, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusCreated)

	response = f.grant.api.request(
		t, http.MethodGet, path, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusOK)

	decodeAuthHTTPJSON(t, response, &output)

	if len(output.Roles) != 2 ||
		output.Roles[0].Slug != "member" ||
		output.Roles[1].Slug != "editor" {
		t.Fatalf("unexpected role ordering: %+v", output.Roles)
	}

	for _, role := range output.Roles {
		if role.OrganizationID != f.grant.organizationID {
			t.Fatal("role from another tenant was returned")
		}
	}

	if output.Roles[1].ID != f.grant.roleID {
		t.Fatal("wrong custom role returned")
	}

	// Remove Editor again.
	response = f.grant.api.request(
		t, http.MethodDelete, f.path, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusOK)

	response = f.grant.api.request(
		t, http.MethodGet, path, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusOK)

	decodeAuthHTTPJSON(t, response, &output)

	if len(output.Roles) != 1 ||
		output.Roles[0].Slug != "member" {
		t.Fatalf("removed role still visible: %+v", output.Roles)
	}
}

func TestListMembershipRolesHTTPValidation(t *testing.T) {
	f := newAssignmentHTTPFixture(t)
	path := membershipRolesListPath(f)

	response := f.grant.api.request(
		t, http.MethodGet, path, "", "",
	)
	requireAuthHTTPStatus(t, response, http.StatusUnauthorized)

	invalidPaths := []string{
		fmt.Sprintf(
			"/v1/organizations/invalid/memberships/%s/roles",
			f.membershipID,
		),
		fmt.Sprintf(
			"/v1/organizations/%s/memberships/invalid/roles",
			f.grant.organizationID,
		),
		fmt.Sprintf(
			"/v1/organizations/%s/memberships/%s/roles",
			f.grant.organizationID,
			uuid.Nil,
		),
	}

	for _, invalidPath := range invalidPaths {
		response = f.grant.api.request(
			t, http.MethodGet, invalidPath, "", f.grant.token,
		)
		requireAuthHTTPStatus(t, response, http.StatusBadRequest)
	}

	unknownMembership := fmt.Sprintf(
		"/v1/organizations/%s/memberships/%s/roles",
		f.grant.organizationID,
		uuid.New(),
	)

	response = f.grant.api.request(
		t, http.MethodGet, unknownMembership, "", f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusNotFound)
}

func TestListMembershipRolesHTTPRequiresMemberRead(t *testing.T) {
	f := newAssignmentHTTPFixture(t)

	result, err := f.grant.api.pool.Exec(
		context.Background(),
		`DELETE FROM role_permissions rp
         USING roles r
         WHERE rp.organization_id = r.organization_id
           AND rp.role_id = r.id
           AND r.organization_id = $1
           AND r.slug = 'owner'
           AND rp.permission_code = 'member:read'`,
		f.grant.organizationID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.RowsAffected() != 1 {
		t.Fatal("expected member:read to be revoked")
	}

	response := f.grant.api.request(
		t,
		http.MethodGet,
		membershipRolesListPath(f),
		"",
		f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusForbidden)
}

func TestListMembershipRolesHTTPRejectsCrossTenant(
	t *testing.T,
) {
	f := newAssignmentHTTPFixture(t)

	slug := uniqueOrganizationSlug()
	cleanupOrganization(t, f.grant.api.pool, slug)

	// The same actor owns both organizations.
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

	membershipB := createAssignmentTestMember(
		t,
		f.grant.api.pool,
		tenantB.ID,
	)

	crossTenantPath := fmt.Sprintf(
		"/v1/organizations/%s/memberships/%s/roles",
		f.grant.organizationID,
		membershipB,
	)

	response = f.grant.api.request(
		t,
		http.MethodGet,
		crossTenantPath,
		"",
		f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusNotFound)
}

func TestListMembershipRolesHTTPHandlesEmptyRoles(t *testing.T) {
	f := newAssignmentHTTPFixture(t)

	// Test only: create a legacy/inconsistent membership
	// without role assignments by bypassing the normal API.
	result, err := f.grant.api.pool.Exec(
		context.Background(),
		`DELETE FROM membership_roles
         WHERE organization_id = $1
           AND membership_id = $2`,
		f.grant.organizationID,
		f.membershipID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.RowsAffected() != 1 {
		t.Fatal("expected one default assignment removed")
	}

	response := f.grant.api.request(
		t,
		http.MethodGet,
		membershipRolesListPath(f),
		"",
		f.grant.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusOK)

	var output membershipRolesHTTPResult
	decodeAuthHTTPJSON(t, response, &output)

	if output.MembershipID != f.membershipID ||
		output.Roles == nil ||
		len(output.Roles) != 0 {
		t.Fatalf("expected empty roles array: %+v", output)
	}
}
