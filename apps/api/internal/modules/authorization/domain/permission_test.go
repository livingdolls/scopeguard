package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestNewPermission(t *testing.T) {
	permission, err := NewPermission(
		"member",
		"invite",
	)
	if err != nil {
		t.Fatal(err)
	}

	if permission.Resource() != "member" {
		t.Fatal("incorrect resource")
	}

	if permission.Action() != "invite" {
		t.Fatal("incorrect action")
	}

	if permission.Code() != "member:invite" {
		t.Fatal("incorrect permission code")
	}
}

func TestPermissionRejectsInvalidResource(t *testing.T) {
	cases := []string{
		"",
		"Member",
		" member",
		"member ",
		"member-name",
		"member:invite",
		strings.Repeat("a", 64),
	}

	for _, resource := range cases {
		_, err := NewPermission(resource, "read")

		if !errors.Is(err, ErrInvalidPermissionResource) {
			t.Errorf(
				"resource %q: expected invalid resource, got %v",
				resource,
				err,
			)
		}
	}
}

func TestPermissionRejectsInvalidAction(t *testing.T) {
	cases := []string{
		"",
		"Invite",
		"invite ",
		"*",
		"invite:all",
		strings.Repeat("a", 64),
	}

	for _, action := range cases {
		_, err := NewPermission("member", action)

		if !errors.Is(err, ErrInvalidPermissionAction) {
			t.Errorf(
				"action %q: expected invalid action, got %v",
				action,
				err,
			)
		}
	}
}

func TestParsePermission(t *testing.T) {
	permission, err := ParsePermission(
		"organization:update",
	)
	if err != nil {
		t.Fatal(err)
	}

	if permission.Resource() != "organization" ||
		permission.Action() != "update" {
		t.Fatal("incorrect parsed permission")
	}
}

func TestParsePermissionRejectsInvalidCode(t *testing.T) {
	cases := []string{
		"",
		"member",
		":invite",
		"member:",
		"member:*",
		"member:invite:all",
		"Member:invite",
	}

	for _, code := range cases {
		_, err := ParsePermission(code)

		if !errors.Is(err, ErrInvalidPermissionCode) {
			t.Errorf(
				"code %q: expected invalid code, got %v",
				code,
				err,
			)
		}
	}
}

func TestBuiltInPermissionsAreValidAndUnique(t *testing.T) {
	seen := make(map[string]bool)

	for _, code := range BuiltInPermissionCodes() {
		if _, err := ParsePermission(code); err != nil {
			t.Fatalf(
				"invalid built-in permission %q: %v",
				code,
				err,
			)
		}

		if seen[code] {
			t.Fatalf(
				"duplicate built-in permission: %s",
				code,
			)
		}

		seen[code] = true
	}
}
