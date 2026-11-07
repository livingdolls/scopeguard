//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type grantRoleHTTPFixture struct {
	api            *authHTTPFixture
	token          string
	organizationID uuid.UUID
	roleID         uuid.UUID
	path           string
}

func newGrantRoleHTTPFixture(t *testing.T) grantRoleHTTPFixture {
	t.Helper()

	f := newAuthHTTPFixture(t)
	user := f.register(t)

	response := f.request(
		t,
		http.MethodPost,
		"/v1/auth/verify-email",
		fmt.Sprintf(`{"token":%q}`, user.verificationToken),
		"",
	)
	requireAuthHTTPStatus(t, response, http.StatusNoContent)

	token, _, err := f.jwtService.Issue(user.id)
	if err != nil {
		t.Fatal(err)
	}

	slug := uniqueOrganizationSlug()
	cleanupOrganization(t, f.pool, slug)

	response = f.request(
		t,
		http.MethodPost,
		"/v1/organizations",
		fmt.Sprintf(`{"name":"Permission Grant Test","slug":%q}`, slug),
		token,
	)
	requireAuthHTTPStatus(t, response, http.StatusCreated)

	var organization struct {
		ID uuid.UUID `json:"id"`
	}
	decodeAuthHTTPJSON(t, response, &organization)

	rolePath := fmt.Sprintf(
		"/v1/organizations/%s/roles",
		organization.ID,
	)

	response = f.request(
		t,
		http.MethodPost,
		rolePath,
		`{"name":"Editor","slug":"editor"}`,
		token,
	)
	requireAuthHTTPStatus(t, response, http.StatusCreated)

	var role struct {
		ID uuid.UUID `json:"id"`
	}
	decodeAuthHTTPJSON(t, response, &role)

	return grantRoleHTTPFixture{
		api:            f,
		token:          token,
		organizationID: organization.ID,
		roleID:         role.ID,
		path:           rolePath + "/" + role.ID.String() + "/permissions",
	}
}

func TestGrantRolePermissionHTTP(t *testing.T) {
	f := newGrantRoleHTTPFixture(t)

	body := `{"permission_code":"member:invite"}`

	// Authentication is mandatory.
	response := f.api.request(
		t, http.MethodPost, f.path, body, "",
	)
	requireAuthHTTPStatus(t, response, http.StatusUnauthorized)

	// A valid permission may be granted to a custom role.
	response = f.api.request(
		t, http.MethodPost, f.path, body, f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusCreated)

	var result struct {
		RoleID         uuid.UUID `json:"role_id"`
		PermissionCode string    `json:"permission_code"`
		Added          bool      `json:"added"`
	}
	decodeAuthHTTPJSON(t, response, &result)

	if result.RoleID != f.roleID ||
		result.PermissionCode != "member:invite" ||
		!result.Added {
		t.Fatalf("unexpected permission grant: %+v", result)
	}

	// Repeated grants must be idempotent.
	response = f.api.request(
		t, http.MethodPost, f.path, body, f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusOK)

	decodeAuthHTTPJSON(t, response, &result)
	if result.Added {
		t.Fatal("duplicate grant must return added=false")
	}

	var count int

	err := f.api.pool.QueryRow(
		context.Background(),
		`SELECT COUNT(*)
         FROM role_permissions
         WHERE organization_id = $1
           AND role_id = $2
           AND permission_code = 'member:invite'`,
		f.organizationID,
		f.roleID,
	).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatalf("expected one assignment, got %d", count)
	}

	// Invalid permission syntax.
	response = f.api.request(
		t,
		http.MethodPost,
		f.path,
		`{"permission_code":"INVALID"}`,
		f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusBadRequest)

	// Well-formed but unknown permission.
	response = f.api.request(
		t,
		http.MethodPost,
		f.path,
		`{"permission_code":"unknown:permission"}`,
		f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusNotFound)

	// Unknown JSON fields are rejected.
	response = f.api.request(
		t,
		http.MethodPost,
		f.path,
		`{"permission_code":"member:invite","is_system":true}`,
		f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusBadRequest)

	// Invalid role identifier.
	invalidRolePath := fmt.Sprintf(
		"/v1/organizations/%s/roles/invalid/permissions",
		f.organizationID,
	)
	response = f.api.request(
		t, http.MethodPost, invalidRolePath, body, f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusBadRequest)

	// Unknown role in the same organization.
	unknownRolePath := fmt.Sprintf(
		"/v1/organizations/%s/roles/%s/permissions",
		f.organizationID,
		uuid.New(),
	)
	response = f.api.request(
		t, http.MethodPost, unknownRolePath, body, f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusNotFound)

	// System roles cannot be modified.
	var ownerRoleID uuid.UUID

	err = f.api.pool.QueryRow(
		context.Background(),
		`SELECT id FROM roles
         WHERE organization_id = $1 AND slug = 'owner'`,
		f.organizationID,
	).Scan(&ownerRoleID)
	if err != nil {
		t.Fatal(err)
	}

	ownerPath := fmt.Sprintf(
		"/v1/organizations/%s/roles/%s/permissions",
		f.organizationID,
		ownerRoleID,
	)

	response = f.api.request(
		t, http.MethodPost, ownerPath, body, f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusConflict)

	// Request bodies exceeding 16 KiB are rejected.
	oversizedBody := fmt.Sprintf(
		`{"permission_code":%q}`,
		strings.Repeat("x", 17*1024),
	)

	response = f.api.request(
		t, http.MethodPost, f.path, oversizedBody, f.token,
	)
	requireAuthHTTPStatus(
		t, response, http.StatusRequestEntityTooLarge,
	)

	// Unsupported content type.
	request := httptest.NewRequest(
		http.MethodPost,
		f.path,
		strings.NewReader(body),
	)
	request.Header.Set("Authorization", "Bearer "+f.token)
	request.Header.Set("Content-Type", "text/plain")

	recorder := httptest.NewRecorder()
	f.api.router.ServeHTTP(recorder, request)

	requireAuthHTTPStatus(
		t, recorder, http.StatusUnsupportedMediaType,
	)
}

func TestGrantRolePermissionHTTPCannotDelegateMissingRight(
	t *testing.T,
) {
	f := newGrantRoleHTTPFixture(t)

	// The actor retains role:update, but loses member:invite.
	result, err := f.api.pool.Exec(
		context.Background(),
		`DELETE FROM role_permissions rp
         USING roles r
         WHERE rp.organization_id = r.organization_id
           AND rp.role_id = r.id
           AND r.organization_id = $1
           AND r.slug = 'owner'
           AND rp.permission_code = 'member:invite'`,
		f.organizationID,
	)
	if err != nil {
		t.Fatal(err)
	}

	if result.RowsAffected() != 1 {
		t.Fatal("expected one revoked actor permission")
	}

	response := f.api.request(
		t,
		http.MethodPost,
		f.path,
		`{"permission_code":"member:invite"}`,
		f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusForbidden)

	var count int

	err = f.api.pool.QueryRow(
		context.Background(),
		`SELECT COUNT(*)
         FROM role_permissions
         WHERE organization_id = $1
           AND role_id = $2
           AND permission_code = 'member:invite'`,
		f.organizationID,
		f.roleID,
	).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	if count != 0 {
		t.Fatal("unauthorized permission was persisted")
	}
}

func TestGrantRolePermissionHTTPRequiresRoleUpdate(t *testing.T) {
	f := newGrantRoleHTTPFixture(t)

	_, err := f.api.pool.Exec(
		context.Background(),
		`DELETE FROM role_permissions rp
         USING roles r
         WHERE rp.organization_id = r.organization_id
           AND rp.role_id = r.id
           AND r.organization_id = $1
           AND r.slug = 'owner'
           AND rp.permission_code = 'role:update'`,
		f.organizationID,
	)
	if err != nil {
		t.Fatal(err)
	}

	response := f.api.request(
		t,
		http.MethodPost,
		f.path,
		`{"permission_code":"member:invite"}`,
		f.token,
	)

	requireAuthHTTPStatus(t, response, http.StatusForbidden)
}

func TestGrantRolePermissionHTTPRejectsCrossTenantRole(
	t *testing.T,
) {
	f := newGrantRoleHTTPFixture(t)

	// The same actor owns a second organization.
	slug := uniqueOrganizationSlug()
	cleanupOrganization(t, f.api.pool, slug)

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

	// Try changing Tenant B's role through Tenant A's URL.
	crossTenantPath := fmt.Sprintf(
		"/v1/organizations/%s/roles/%s/permissions",
		f.organizationID,
		roleB.ID,
	)

	response = f.api.request(
		t,
		http.MethodPost,
		crossTenantPath,
		`{"permission_code":"member:invite"}`,
		f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusNotFound)

	var count int

	err := f.api.pool.QueryRow(
		context.Background(),
		`SELECT COUNT(*)
         FROM role_permissions
         WHERE organization_id = $1
           AND role_id = $2
           AND permission_code = 'member:invite'`,
		tenantB.ID,
		roleB.ID,
	).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	if count != 0 {
		t.Fatal("cross-tenant grant modified Tenant B")
	}
}
