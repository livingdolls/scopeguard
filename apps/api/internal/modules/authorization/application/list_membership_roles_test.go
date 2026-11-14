package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type fakeMembershipRolesReader struct {
	calls int
	roles []RoleListItem
	err   error
}

func (f *fakeMembershipRolesReader) ListByMembership(
	ctx context.Context,
	organizationID uuid.UUID,
	membershipID uuid.UUID,
) ([]RoleListItem, error) {
	f.calls++
	return f.roles, f.err
}

func TestListMembershipRolesSuccess(t *testing.T) {
	expectedRole := RoleListItem{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Name:           "Member",
		Slug:           "member",
		IsSystem:       true,
	}

	repository := &fakeMembershipRolesReader{
		roles: []RoleListItem{expectedRole},
	}

	output, err := NewListMembershipRolesService(repository).
		Execute(
			context.Background(),
			expectedRole.OrganizationID,
			uuid.New(),
		)
	if err != nil {
		t.Fatal(err)
	}

	if repository.calls != 1 ||
		len(output) != 1 ||
		output[0].ID != expectedRole.ID {
		t.Fatalf("unexpected roles: %+v", output)
	}
}

func TestListMembershipRolesInvalidInput(t *testing.T) {
	tests := []struct {
		name         string
		organization uuid.UUID
		membership   uuid.UUID
		expected     error
	}{
		{
			name:         "nil organization",
			organization: uuid.Nil,
			membership:   uuid.New(),
			expected:     ErrRoleOrganizationNotFound,
		},
		{
			name:         "nil membership",
			organization: uuid.New(),
			membership:   uuid.Nil,
			expected:     ErrMembershipNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reader := &fakeMembershipRolesReader{}

			_, err := NewListMembershipRolesService(reader).
				Execute(
					context.Background(),
					tc.organization,
					tc.membership,
				)

			if !errors.Is(err, tc.expected) {
				t.Fatalf("expected %v, got %v", tc.expected, err)
			}

			if reader.calls != 0 {
				t.Fatal("invalid input reached repository")
			}
		})
	}
}

func TestListMembershipRolesCanceledContext(t *testing.T) {
	reader := &fakeMembershipRolesReader{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewListMembershipRolesService(reader).
		Execute(ctx, uuid.New(), uuid.New())

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}

	if reader.calls != 0 {
		t.Fatal("canceled request reached repository")
	}
}

func TestListMembershipRolesEmptyResult(t *testing.T) {
	reader := &fakeMembershipRolesReader{}

	output, err := NewListMembershipRolesService(reader).
		Execute(context.Background(), uuid.New(), uuid.New())

	if err != nil {
		t.Fatal(err)
	}

	if output == nil || len(output) != 0 {
		t.Fatalf("expected empty non-nil roles slice: %+v", output)
	}
}
