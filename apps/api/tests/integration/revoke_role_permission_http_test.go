//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

// Prepare a custom role with member:invite assigned.
func newRevokeRoleHTTPFixture(
	t *testing.T,
) (grantRoleHTTPFixture, string) {
	t.Helper()

	f := newGrantRoleHTTPFixture(t)

	response := f.api.request(
		t,
		http.MethodPost,
		f.path,
		`{"permission_code":"member:invite"}`,
		f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusCreated)

	revokePath := f.path + "/member:invite"

	return f, revokePath
}

func TestRevokeRolePermissionHTTPLifecycle(t *testing.T) {
	f, path := newRevokeRoleHTTPFixture(t)

	// First revoke removes the assignment.
	response := f.api.request(
		t, http.MethodDelete, path, "", f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusOK)

	var output struct {
		RoleID         uuid.UUID `json:"role_id"`
		PermissionCode string    `json:"permission_code"`
		Removed        bool      `json:"removed"`
	}

	decodeAuthHTTPJSON(t, response, &output)

	if output.RoleID != f.roleID ||
		output.PermissionCode != "member:invite" ||
		!output.Removed {
		t.Fatalf("unexpected first revoke: %+v", output)
	}

	// Repeating an authorized revoke is a no-op.
	response = f.api.request(
		t, http.MethodDelete, path, "", f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusOK)

	decodeAuthHTTPJSON(t, response, &output)

	if output.Removed {
		t.Fatal("second revoke must return removed=false")
	}

	var count int

	err := f.api.pool.QueryRow(
		context.Background(),
		`
        SELECT COUNT(*)
        FROM role_permissions
        WHERE organization_id = $1
          AND role_id = $2
          AND permission_code = 'member:invite'
        `,
		f.organizationID,
		f.roleID,
	).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	if count != 0 {
		t.Fatal("revoked permission remains in database")
	}
}

func TestRevokeRolePermissionHTTPValidation(t *testing.T) {
	f, path := newRevokeRoleHTTPFixture(t)

	// Authentication is required.
	response := f.api.request(
		t, http.MethodDelete, path, "", "",
	)
	requireAuthHTTPStatus(t, response, http.StatusUnauthorized)

	// Invalid organization ID.
	response = f.api.request(
		t,
		http.MethodDelete,
		"/v1/organizations/invalid/roles/"+
			f.roleID.String()+"/permissions/member:invite",
		"",
		f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusBadRequest)

	// Invalid role ID.
	response = f.api.request(
		t,
		http.MethodDelete,
		"/v1/organizations/"+f.organizationID.String()+
			"/roles/invalid/permissions/member:invite",
		"",
		f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusBadRequest)

	// Invalid permission syntax.
	response = f.api.request(
		t,
		http.MethodDelete,
		f.path+"/INVALID",
		"",
		f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusBadRequest)

	// Well-formed but unknown permission.
	response = f.api.request(
		t,
		http.MethodDelete,
		f.path+"/unknown:permission",
		"",
		f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusNotFound)

	// Unknown role belonging to no organization.
	unknownRolePath := fmt.Sprintf(
		"/v1/organizations/%s/roles/%s/permissions/member:invite",
		f.organizationID,
		uuid.New(),
	)

	response = f.api.request(
		t,
		http.MethodDelete,
		unknownRolePath,
		"",
		f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusNotFound)

	// System roles are immutable.
	var ownerRoleID uuid.UUID

	err := f.api.pool.QueryRow(
		context.Background(),
		`
        SELECT id
        FROM roles
        WHERE organization_id = $1
          AND slug = 'owner'
        `,
		f.organizationID,
	).Scan(&ownerRoleID)
	if err != nil {
		t.Fatal(err)
	}

	ownerPath := fmt.Sprintf(
		"/v1/organizations/%s/roles/%s/permissions/member:invite",
		f.organizationID,
		ownerRoleID,
	)

	response = f.api.request(
		t,
		http.MethodDelete,
		ownerPath,
		"",
		f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusConflict)
}

func TestRevokeRolePermissionHTTPRequiresPermissions(
	t *testing.T,
) {
	tests := []struct {
		name            string
		actorPermission string
	}{
		{
			name:            "missing role update",
			actorPermission: "role:update",
		},
		{
			name:            "missing delegated permission",
			actorPermission: "member:invite",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, path := newRevokeRoleHTTPFixture(t)

			// Remove one of the permissions from the actor's
			// owner role, but keep the target role assignment.
			result, err := f.api.pool.Exec(
				context.Background(),
				`
                DELETE FROM role_permissions rp
                USING roles r
                WHERE rp.organization_id = r.organization_id
                  AND rp.role_id = r.id
                  AND r.organization_id = $1
                  AND r.slug = 'owner'
                  AND rp.permission_code = $2
                `,
				f.organizationID,
				tt.actorPermission,
			)
			if err != nil {
				t.Fatal(err)
			}

			if result.RowsAffected() != 1 {
				t.Fatal("expected one actor permission removed")
			}

			// The existing token must not bypass
			// effective database permissions.
			response := f.api.request(
				t,
				http.MethodDelete,
				path,
				"",
				f.token,
			)
			requireAuthHTTPStatus(
				t, response, http.StatusForbidden,
			)

			var count int

			err = f.api.pool.QueryRow(
				context.Background(),
				`
                SELECT COUNT(*)
                FROM role_permissions
                WHERE organization_id = $1
                  AND role_id = $2
                  AND permission_code = 'member:invite'
                `,
				f.organizationID,
				f.roleID,
			).Scan(&count)
			if err != nil {
				t.Fatal(err)
			}

			if count != 1 {
				t.Fatal("unauthorized revoke changed target role")
			}
		})
	}
}

func TestRevokeRolePermissionHTTPRejectsCrossTenantRole(
	t *testing.T,
) {
	f, _ := newRevokeRoleHTTPFixture(t)

	// Create another organization with the same actor.
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

	// Create a custom role in Tenant B.
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

	// Grant member:invite to role B.
	grantPathB := rolePathB + "/" +
		roleB.ID.String() + "/permissions"

	response = f.api.request(
		t,
		http.MethodPost,
		grantPathB,
		`{"permission_code":"member:invite"}`,
		f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusCreated)

	// Attempt to revoke Tenant B's role using Tenant A's URL.
	crossTenantPath := fmt.Sprintf(
		"/v1/organizations/%s/roles/%s/permissions/member:invite",
		f.organizationID,
		roleB.ID,
	)

	response = f.api.request(
		t,
		http.MethodDelete,
		crossTenantPath,
		"",
		f.token,
	)
	requireAuthHTTPStatus(t, response, http.StatusNotFound)

	// Tenant B's permission must remain unchanged.
	var count int

	err := f.api.pool.QueryRow(
		context.Background(),
		`
        SELECT COUNT(*)
        FROM role_permissions
        WHERE organization_id = $1
          AND role_id = $2
          AND permission_code = 'member:invite'
        `,
		tenantB.ID,
		roleB.ID,
	).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}

	if count != 1 {
		t.Fatal("cross-tenant revoke removed Tenant B permission")
	}
}
