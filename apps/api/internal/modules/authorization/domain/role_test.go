package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewRole(t *testing.T) {
	organizationID := uuid.New()

	role, err := NewRole(
		uuid.New(),
		organizationID,
		"Content Editor",
		"content_editor",
		false,
		time.Now(),
	)
	if err != nil {
		t.Fatal(err)
	}

	if role.OrganizationID() != organizationID {
		t.Fatal("incorrect organization ID")
	}

	if role.Slug() != "content_editor" {
		t.Fatal("incorrect role slug")
	}

	if role.IsSystem() {
		t.Fatal("custom role must not be system")
	}
}

func TestNewRoleSystemRoles(t *testing.T) {
	for _, slug := range []string{
		RoleOwner,
		RoleMember,
	} {
		role, err := NewRole(
			uuid.New(),
			uuid.New(),
			"System Role",
			slug,
			true,
			time.Now(),
		)

		if err != nil {
			t.Fatal(err)
		}

		if !role.IsSystem() {
			t.Fatal("expected system role")
		}
	}
}

func TestNewRoleRejectsReservedCustomSlug(t *testing.T) {
	for _, slug := range []string{
		RoleOwner,
		RoleMember,
	} {
		_, err := NewRole(
			uuid.New(),
			uuid.New(),
			"Custom Role",
			slug,
			false,
			time.Now(),
		)

		if !errors.Is(err, ErrInvalidRoleSystemFlag) {
			t.Fatalf("expected reserved role rejection: %v", err)
		}
	}
}

func TestNewRoleRejectsInvalidTenant(t *testing.T) {
	_, err := NewRole(
		uuid.New(),
		uuid.Nil,
		"Editor",
		"editor",
		false,
		time.Now(),
	)

	if !errors.Is(err, ErrInvalidRoleOrganizationID) {
		t.Fatalf("expected invalid organization ID: %v", err)
	}
}

func TestNewRoleRejectsInvalidSlug(t *testing.T) {
	cases := []string{
		"",
		"Admin",
		"admin-role",
		"admin:*",
		" admin",
	}

	for _, slug := range cases {
		_, err := NewRole(
			uuid.New(),
			uuid.New(),
			"Test Role",
			slug,
			false,
			time.Now(),
		)

		if !errors.Is(err, ErrInvalidRoleSlug) {
			t.Fatalf("slug %q: unexpected error %v", slug, err)
		}
	}
}
