//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"
)

func TestListRolePermissionsHTTPLifecycle(t *testing.T) {
	f := newGrantRoleHTTPFixture(t)

	// A newly created custom role has no permissions.
	response := f.api.request(
		t, http.MethodGet, f.path, "", f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusOK)

	var output struct {
		RoleID      uuid.UUID `json:"role_id"`
		Permissions []string  `json:"permissions"`
	}
	decodeAuthHTTPJSON(t, response, &output)

	if output.RoleID != f.roleID ||
		output.Permissions == nil ||
		len(output.Permissions) != 0 {
		t.Fatalf("expected empty permissions: %+v", output)
	}

	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("role permission response must not be cached")
	}

	// Grant two permissions in reverse alphabetical order.
	for _, code := range []string{
		"organization:read",
		"member:invite",
	} {
		response = f.api.request(
			t,
			http.MethodPost,
			f.path,
			fmt.Sprintf(`{"permission_code":%q}`, code),
			f.token,
		)
		requireAuthHTTPStatus(t, response, http.StatusCreated)
	}

	response = f.api.request(
		t, http.MethodGet, f.path, "", f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusOK)

	decodeAuthHTTPJSON(t, response, &output)

	expected := []string{
		"member:invite",
		"organization:read",
	}

	if !slices.Equal(output.Permissions, expected) {
		t.Fatalf(
			"expected %v, got %v",
			expected,
			output.Permissions,
		)
	}

	// Revoke one permission.
	response = f.api.request(
		t,
		http.MethodDelete,
		f.path+"/member:invite",
		"",
		f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusOK)

	response = f.api.request(
		t, http.MethodGet, f.path, "", f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusOK)

	decodeAuthHTTPJSON(t, response, &output)

	if !slices.Equal(
		output.Permissions,
		[]string{"organization:read"},
	) {
		t.Fatalf(
			"revoked permission is still visible: %v",
			output.Permissions,
		)
	}
}

func TestListRolePermissionsHTTPValidation(t *testing.T) {
	f := newGrantRoleHTTPFixture(t)

	response := f.api.request(
		t, http.MethodGet, f.path, "", "",
	)
	requireAuthHTTPStatus(t, response, http.StatusUnauthorized)

	invalidPaths := []string{
		"/v1/organizations/invalid/roles/" +
			f.roleID.String() + "/permissions",
		"/v1/organizations/" + f.organizationID.String() +
			"/roles/invalid/permissions",
		"/v1/organizations/" + f.organizationID.String() +
			"/roles/" + uuid.Nil.String() + "/permissions",
	}

	for _, path := range invalidPaths {
		response = f.api.request(
			t, http.MethodGet, path, "", f.token,
		)
		requireAuthHTTPStatus(t, response, http.StatusBadRequest)
	}

	unknownRole := fmt.Sprintf(
		"/v1/organizations/%s/roles/%s/permissions",
		f.organizationID,
		uuid.New(),
	)

	response = f.api.request(
		t, http.MethodGet, unknownRole, "", f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusNotFound)
}

func TestListRolePermissionsHTTPRequiresRoleRead(t *testing.T) {
	f := newGrantRoleHTTPFixture(t)

	result, err := f.api.pool.Exec(
		context.Background(),
		`DELETE FROM role_permissions rp
         USING roles r
         WHERE rp.organization_id = r.organization_id
           AND rp.role_id = r.id
           AND r.organization_id = $1
           AND r.slug = 'owner'
           AND rp.permission_code = 'role:read'`,
		f.organizationID,
	)
	if err != nil {
		t.Fatal(err)
	}

	if result.RowsAffected() != 1 {
		t.Fatal("expected role:read to be revoked")
	}

	response := f.api.request(
		t, http.MethodGet, f.path, "", f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusForbidden)
}

func TestListRolePermissionsHTTPRejectsCrossTenantRole(
	t *testing.T,
) {
	f := newGrantRoleHTTPFixture(t)

	slug := uniqueOrganizationSlug()
	cleanupOrganization(t, f.api.pool, slug)

	// The same actor owns both organizations.
	response := f.api.request(
		t,
		http.MethodPost,
		"/v1/organizations",
		fmt.Sprintf(
			`{"name":"Second Organization","slug":%q}`,
			slug,
		),
		f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusCreated)

	var tenantB struct {
		ID uuid.UUID `json:"id"`
	}
	decodeAuthHTTPJSON(t, response, &tenantB)

	rolePathB := "/v1/organizations/" +
		tenantB.ID.String() + "/roles"

	response = f.api.request(
		t,
		http.MethodPost,
		rolePathB,
		`{"name":"Auditor","slug":"auditor"}`,
		f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusCreated)

	var roleB struct {
		ID uuid.UUID `json:"id"`
	}
	decodeAuthHTTPJSON(t, response, &roleB)

	// Request a Tenant B role using Tenant A's organization ID.
	crossTenantPath := fmt.Sprintf(
		"/v1/organizations/%s/roles/%s/permissions",
		f.organizationID,
		roleB.ID,
	)

	response = f.api.request(
		t,
		http.MethodGet,
		crossTenantPath,
		"",
		f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusNotFound)
}
