//go:build integration

package integration

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestAuthorizationPermissionCatalogHTTP(t *testing.T) {
	f := newAuthHTTPFixture(t)
	user := f.register(t)

	// Activate the registered user through HTTP.
	verificationBody := fmt.Sprintf(
		`{"token":%q}`,
		user.verificationToken,
	)

	response := f.request(
		t,
		http.MethodPost,
		"/v1/auth/verify-email",
		verificationBody,
		"",
	)
	requireAuthHTTPStatus(
		t, response, http.StatusNoContent,
	)

	// Issue a correctly signed test JWT for this active user.
	token, _, err := f.jwtService.Issue(user.id)
	if err != nil {
		t.Fatal(err)
	}

	slug := uniqueOrganizationSlug()
	cleanupOrganization(t, f.pool, slug)

	// Create organization using production HTTP routing.
	response = f.request(
		t,
		http.MethodPost,
		"/v1/organizations",
		fmt.Sprintf(
			`{"name":"Permission Test Organization","slug":%q}`,
			slug,
		),
		token,
	)
	requireAuthHTTPStatus(
		t, response, http.StatusCreated,
	)

	var organization struct {
		ID uuid.UUID `json:"id"`
	}

	decodeAuthHTTPJSON(t, response, &organization)

	path := "/v1/organizations/" +
		organization.ID.String() +
		"/permissions"

	// Owner has permission:read.
	response = f.request(
		t, http.MethodGet, path, "", token,
	)
	requireAuthHTTPStatus(
		t, response, http.StatusOK,
	)

	var catalog struct {
		Permissions []struct {
			Code string `json:"code"`
		} `json:"permissions"`
	}

	decodeAuthHTTPJSON(t, response, &catalog)

	if len(catalog.Permissions) != 12 {
		t.Fatalf(
			"expected 12 catalog permissions, got %d",
			len(catalog.Permissions),
		)
	}

	// No JWT.
	response = f.request(
		t, http.MethodGet, path, "", "",
	)
	requireAuthHTTPStatus(
		t, response, http.StatusUnauthorized,
	)

	// JWT is valid, but organization ID is malformed.
	response = f.request(
		t,
		http.MethodGet,
		"/v1/organizations/invalid/permissions",
		"",
		token,
	)
	requireAuthHTTPStatus(
		t, response, http.StatusBadRequest,
	)

	// JWT is valid, but user has no membership there.
	response = f.request(
		t,
		http.MethodGet,
		"/v1/organizations/"+uuid.NewString()+"/permissions",
		"",
		token,
	)
	requireAuthHTTPStatus(
		t, response, http.StatusForbidden,
	)
}
