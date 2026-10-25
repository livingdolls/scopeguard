package domain

import (
	"slices"
	"testing"
)

func TestDefaultSystemRolePermissions(t *testing.T) {
	owner := DefaultSystemRolePermissions(RoleOwner)
	member := DefaultSystemRolePermissions(RoleMember)

	if len(owner) != 12 {
		t.Fatalf(
			"expected 12 owner permissions, got %d",
			len(owner),
		)
	}

	if len(member) != 4 {
		t.Fatalf(
			"expected 4 member permissions, got %d",
			len(member),
		)
	}

	catalog := BuiltInPermissionCodes()

	for _, policy := range [][]string{owner, member} {
		seen := make(map[string]bool)

		for _, code := range policy {
			if _, err := ParsePermission(code); err != nil {
				t.Fatalf("invalid permission %q: %v", code, err)
			}

			if !slices.Contains(catalog, code) {
				t.Fatalf("permission %q not in catalog", code)
			}

			if seen[code] {
				t.Fatalf("duplicate permission %q", code)
			}
			seen[code] = true
		}
	}

	// Member must not receive administrative permissions.
	for _, forbidden := range []string{
		PermissionRoleCreate,
		PermissionMemberRoleAssign,
		PermissionOrganizationDelete,
	} {
		if slices.Contains(member, forbidden) {
			t.Fatalf(
				"member must not have %q",
				forbidden,
			)
		}
	}

	for _, code := range member {
		if !slices.Contains(owner, code) {
			t.Fatalf(
				"owner is missing member permission %q",
				code,
			)
		}
	}
}

func TestUnknownRoleHasNoDefaultPermissions(t *testing.T) {
	if permissions := DefaultSystemRolePermissions("editor"); len(permissions) != 0 {
		t.Fatal("custom roles must not receive system defaults")
	}
}

func TestSystemRolePermissionsCannotBeMutatedGlobally(t *testing.T) {
	permissions := DefaultSystemRolePermissions(RoleMember)
	permissions[0] = PermissionOrganizationDelete

	fresh := DefaultSystemRolePermissions(RoleMember)

	if fresh[0] != PermissionOrganizationRead {
		t.Fatal("default policy was unexpectedly mutated")
	}
}
