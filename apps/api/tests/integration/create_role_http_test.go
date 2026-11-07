//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestAuthorizationCreateRoleHTTP(t *testing.T) {
	f := newAuthHTTPFixture(t)
	user := f.register(t)

	// Verify the registered account through the production API.
	response := f.request(t, http.MethodPost, "/v1/auth/verify-email",
		fmt.Sprintf(`{"token":%q}`, user.verificationToken), "")
	requireAuthHTTPStatus(t, response, http.StatusNoContent)

	token, _, err := f.jwtService.Issue(user.id)
	if err != nil {
		t.Fatal(err)
	}

	slug := uniqueOrganizationSlug()
	cleanupOrganization(t, f.pool, slug)
	response = f.request(t, http.MethodPost, "/v1/organizations",
		fmt.Sprintf(`{"name":"Create Role Test","slug":%q}`, slug), token)
	requireAuthHTTPStatus(t, response, http.StatusCreated)

	var organization struct {
		ID uuid.UUID `json:"id"`
	}
	decodeAuthHTTPJSON(t, response, &organization)

	path := "/v1/organizations/" + organization.ID.String() + "/roles"
	validBody := `{"name":"Content Editor","slug":"content_editor"}`

	// Authentication and a valid organization ID are required.
	response = f.request(t, http.MethodPost, path, validBody, "")
	requireAuthHTTPStatus(t, response, http.StatusUnauthorized)
	response = f.request(t, http.MethodPost, "/v1/organizations/invalid/roles", validBody, token)
	requireAuthHTTPStatus(t, response, http.StatusBadRequest)

	response = f.request(t, http.MethodPost, path, validBody, token)
	requireAuthHTTPStatus(t, response, http.StatusCreated)

	var role struct {
		ID             uuid.UUID `json:"id"`
		OrganizationID uuid.UUID `json:"organization_id"`
		Name           string    `json:"name"`
		Slug           string    `json:"slug"`
		IsSystem       bool      `json:"is_system"`
	}
	decodeAuthHTTPJSON(t, response, &role)
	if role.ID == uuid.Nil {
		t.Fatal("created role must have a valid ID")
	}
	if role.OrganizationID != organization.ID {
		t.Fatal("created role belongs to incorrect tenant")
	}
	if role.Name != "Content Editor" || role.Slug != "content_editor" || role.IsSystem {
		t.Fatalf("unexpected custom role response: %+v", role)
	}

	response = f.request(t, http.MethodPost, path, validBody, token)
	requireAuthHTTPStatus(t, response, http.StatusConflict)
	response = f.request(t, http.MethodPost, path, `{"name":"Owner","slug":"owner"}`, token)
	requireAuthHTTPStatus(t, response, http.StatusBadRequest)
	response = f.request(t, http.MethodPost, path, `{"name":"Admin","slug":"admin","is_system":true}`, token)
	requireAuthHTTPStatus(t, response, http.StatusBadRequest)

	var storedCount int
	err = f.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM roles
		 WHERE id = $1 AND organization_id = $2
		   AND slug = 'content_editor' AND is_system = FALSE`,
		role.ID, organization.ID,
	).Scan(&storedCount)
	if err != nil {
		t.Fatal(err)
	}
	if storedCount != 1 {
		t.Fatal("expected exactly one persisted role")
	}

	// Change effective permissions after the JWT has already been issued.
	result, err := f.pool.Exec(context.Background(),
		`DELETE FROM role_permissions rp
		 USING roles r
		 WHERE rp.organization_id = r.organization_id
		   AND rp.role_id = r.id
		   AND r.organization_id = $1
		   AND r.slug = 'owner'
		   AND rp.permission_code = 'role:create'`,
		organization.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.RowsAffected() != 1 {
		t.Fatal("expected one permission revocation")
	}

	// The same JWT still authenticates, but PostgreSQL no longer authorizes role creation.
	response = f.request(t, http.MethodGet, "/v1/auth/me", "", token)
	requireAuthHTTPStatus(t, response, http.StatusOK)
	response = f.request(t, http.MethodPost, path, `{"name":"Denied Role","slug":"denied_role"}`, token)
	requireAuthHTTPStatus(t, response, http.StatusForbidden)

	err = f.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM roles WHERE organization_id = $1 AND slug = 'denied_role'`,
		organization.ID,
	).Scan(&storedCount)
	if err != nil {
		t.Fatal(err)
	}
	if storedCount != 0 {
		t.Fatal("unauthorized role was persisted")
	}
}
