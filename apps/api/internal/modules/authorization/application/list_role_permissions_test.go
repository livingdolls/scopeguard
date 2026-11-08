package application

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
)

type fakeRolePermissionReader struct {
	calls          int
	organizationID uuid.UUID
	roleID         uuid.UUID
	permissions    []string
	err            error
}

func (f *fakeRolePermissionReader) ListByRole(
	ctx context.Context,
	organizationID uuid.UUID,
	roleID uuid.UUID,
) ([]string, error) {
	f.calls++
	f.organizationID = organizationID
	f.roleID = roleID
	return f.permissions, f.err
}

func TestListRolePermissionsSuccess(t *testing.T) {
	repository := &fakeRolePermissionReader{
		permissions: []string{
			"member:invite",
			"role:read",
		},
	}

	organizationID := uuid.New()
	roleID := uuid.New()

	service := NewListRolePermissionsService(repository)

	output, err := service.Execute(
		context.Background(),
		organizationID,
		roleID,
	)
	if err != nil {
		t.Fatal(err)
	}

	if repository.calls != 1 ||
		repository.organizationID != organizationID ||
		repository.roleID != roleID {
		t.Fatal("repository received incorrect input")
	}

	if !slices.Equal(output, repository.permissions) {
		t.Fatalf("unexpected permissions: %v", output)
	}
}

func TestListRolePermissionsEmptyResult(t *testing.T) {
	repository := &fakeRolePermissionReader{}

	output, err := NewListRolePermissionsService(repository).
		Execute(context.Background(), uuid.New(), uuid.New())

	if err != nil {
		t.Fatal(err)
	}

	if output == nil || len(output) != 0 {
		t.Fatal("expected an empty non-nil slice")
	}
}

func TestListRolePermissionsInvalidInput(t *testing.T) {
	cases := []struct {
		name         string
		organization uuid.UUID
		role         uuid.UUID
		expected     error
	}{
		{
			name:         "nil organization",
			organization: uuid.Nil,
			role:         uuid.New(),
			expected:     ErrRoleOrganizationNotFound,
		},
		{
			name:         "nil role",
			organization: uuid.New(),
			role:         uuid.Nil,
			expected:     ErrTargetRoleNotFound,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repository := &fakeRolePermissionReader{}

			_, err := NewListRolePermissionsService(repository).
				Execute(
					context.Background(),
					tc.organization,
					tc.role,
				)

			if !errors.Is(err, tc.expected) {
				t.Fatalf("expected %v, got %v", tc.expected, err)
			}

			if repository.calls != 0 {
				t.Fatal("invalid input reached repository")
			}
		})
	}
}

func TestListRolePermissionsCanceledContext(t *testing.T) {
	repository := &fakeRolePermissionReader{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewListRolePermissionsService(repository).
		Execute(ctx, uuid.New(), uuid.New())

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context, got %v", err)
	}

	if repository.calls != 0 {
		t.Fatal("canceled request reached repository")
	}
}
