package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type fakeMembershipRoleRepository struct {
	calls    int
	assigned bool
	err      error

	actorID      uuid.UUID
	organization uuid.UUID
	membership   uuid.UUID
	role         uuid.UUID
}

func (f *fakeMembershipRoleRepository) Assign(
	ctx context.Context,
	actorID uuid.UUID,
	orgID uuid.UUID,
	membershipID uuid.UUID,
	roleID uuid.UUID,
) (bool, error) {
	f.calls++
	f.actorID = actorID
	f.organization = orgID
	f.membership = membershipID
	f.role = roleID
	return f.assigned, f.err
}

func validAssignInput() AssignRoleToMemberInput {
	return AssignRoleToMemberInput{
		ActorUserID:    uuid.New(),
		OrganizationID: uuid.New(),
		MembershipID:   uuid.New(),
		RoleID:         uuid.New(),
	}
}

func TestAssignRoleToMemberSuccess(t *testing.T) {
	repo := &fakeMembershipRoleRepository{assigned: true}
	service := NewAssignRoleToMemberService(repo)
	input := validAssignInput()

	result, err := service.Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Assigned || repo.calls != 1 {
		t.Fatal("expected one successful assignment")
	}

	if repo.actorID != input.ActorUserID ||
		repo.organization != input.OrganizationID ||
		repo.membership != input.MembershipID ||
		repo.role != input.RoleID {
		t.Fatal("repository received incorrect identifiers")
	}
}

func TestAssignRoleToMemberIdempotent(t *testing.T) {
	repo := &fakeMembershipRoleRepository{assigned: false}
	service := NewAssignRoleToMemberService(repo)

	output, err := service.Execute(
		context.Background(),
		validAssignInput(),
	)

	if err != nil || output.Assigned || repo.calls != 1 {
		t.Fatalf("expected authorized no-op, got %+v, %v", output, err)
	}
}

func TestAssignRoleToMemberInvalidInput(t *testing.T) {
	cases := []struct {
		name string
		edit func(*AssignRoleToMemberInput)
	}{
		{"actor", func(i *AssignRoleToMemberInput) {
			i.ActorUserID = uuid.Nil
		}},
		{"organization", func(i *AssignRoleToMemberInput) {
			i.OrganizationID = uuid.Nil
		}},
		{"membership", func(i *AssignRoleToMemberInput) {
			i.MembershipID = uuid.Nil
		}},
		{"role", func(i *AssignRoleToMemberInput) {
			i.RoleID = uuid.Nil
		}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeMembershipRoleRepository{}
			input := validAssignInput()
			tt.edit(&input)

			_, err := NewAssignRoleToMemberService(repo).
				Execute(context.Background(), input)

			if !errors.Is(err, ErrPermissionAssignmentForbidden) {
				t.Fatalf("expected forbidden, got %v", err)
			}
			if repo.calls != 0 {
				t.Fatal("invalid input reached repository")
			}
		})
	}
}

func TestAssignRoleToMemberCanceledContext(t *testing.T) {
	repo := &fakeMembershipRoleRepository{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewAssignRoleToMemberService(repo).
		Execute(ctx, validAssignInput())

	if !errors.Is(err, context.Canceled) || repo.calls != 0 {
		t.Fatalf("unexpected canceled result: %v", err)
	}
}
