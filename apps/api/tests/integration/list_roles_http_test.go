//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestAuthorizationListRolesHTTP(t *testing.T) {
	f := newAuthHTTPFixture(t)
	user := f.register(t)

	// Activate the user.
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

	// Create an organization using production routing.
	response = f.request(
		t,
		http.MethodPost,
		"/v1/organizations",
		fmt.Sprintf(
			`{"name":"Role Listing Test","slug":%q}`,
			slug,
		),
		token,
	)
	requireAuthHTTPStatus(t, response, http.StatusCreated)

	var organization struct {
		ID uuid.UUID `json:"id"`
	}

	decodeAuthHTTPJSON(t, response, &organization)

	// Seed one custom role to verify tenant-scoped listing.
	_, err = f.pool.Exec(
		context.Background(),
		`INSERT INTO roles (
            id, organization_id, name, slug, is_system
        ) VALUES ($1, $2, 'Auditor', 'auditor', FALSE)`,
		uuid.New(),
		organization.ID,
	)
	if err != nil {
		t.Fatal(err)
	}

	path := "/v1/organizations/" +
		organization.ID.String() + "/roles"

	response = f.request(
		t, http.MethodGet, path, "", token,
	)
	requireAuthHTTPStatus(t, response, http.StatusOK)

	var result struct {
		Roles []struct {
			ID             uuid.UUID `json:"id"`
			OrganizationID uuid.UUID `json:"organization_id"`
			Slug           string    `json:"slug"`
			IsSystem       bool      `json:"is_system"`
		} `json:"roles"`
	}

	decodeAuthHTTPJSON(t, response, &result)

	if len(result.Roles) != 3 {
		t.Fatalf(
			"expected owner, member and auditor; got %d roles",
			len(result.Roles),
		)
	}

	seen := make(map[string]bool)

	for _, role := range result.Roles {
		if role.OrganizationID != organization.ID {
			t.Fatal("cross-tenant role returned")
		}

		if seen[role.Slug] {
			t.Fatal("duplicate role in response")
		}

		seen[role.Slug] = true
	}

	for _, slug := range []string{"owner", "member", "auditor"} {
		if !seen[slug] {
			t.Fatalf("missing role %s", slug)
		}
	}

	// No authentication.
	response = f.request(
		t, http.MethodGet, path, "", "",
	)
	requireAuthHTTPStatus(t, response, http.StatusUnauthorized)

	// Invalid organization identifier.
	response = f.request(
		t,
		http.MethodGet,
		"/v1/organizations/invalid/roles",
		"",
		token,
	)
	requireAuthHTTPStatus(t, response, http.StatusBadRequest)

	// A different tenant without membership must be denied.
	response = f.request(
		t,
		http.MethodGet,
		"/v1/organizations/"+uuid.NewString()+"/roles",
		"",
		token,
	)
	requireAuthHTTPStatus(t, response, http.StatusForbidden)
}
