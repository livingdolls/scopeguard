package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type fakeMembershipRoleRemover struct {
	calls   int
	removed bool
	err     error

	actorID        uuid.UUID
	organizationID uuid.UUID
	membershipID   uuid.UUID
	roleID         uuid.UUID
}

func (f *fakeMembershipRoleRemover) Remove(
	ctx context.Context,
	actorID uuid.UUID,
	organizationID uuid.UUID,
	membershipID uuid.UUID,
	roleID uuid.UUID,
) (bool, error) {
	f.calls++
	f.actorID = actorID
	f.organizationID = organizationID
	f.membershipID = membershipID
	f.roleID = roleID

	return f.removed, f.err
}

func validRemoveInput() RemoveRoleFromMemberInput {
	return RemoveRoleFromMemberInput{
		ActorUserID:    uuid.New(),
		OrganizationID: uuid.New(),
		MembershipID:   uuid.New(),
		RoleID:         uuid.New(),
	}
}

func TestRemoveRoleFromMemberSuccess(t *testing.T) {
	repo := &fakeMembershipRoleRemover{removed: true}
	input := validRemoveInput()

	output, err := NewRemoveRoleFromMemberService(repo).
		Execute(context.Background(), input)

	if err != nil {
		t.Fatal(err)
	}

	if !output.Removed || repo.calls != 1 {
		t.Fatal("expected successful role removal")
	}

	if repo.actorID != input.ActorUserID ||
		repo.organizationID != input.OrganizationID ||
		repo.membershipID != input.MembershipID ||
		repo.roleID != input.RoleID {
		t.Fatal("incorrect repository parameters")
	}
}

func TestRemoveRoleFromMemberIdempotent(t *testing.T) {
	repo := &fakeMembershipRoleRemover{removed: false}

	output, err := NewRemoveRoleFromMemberService(repo).
		Execute(context.Background(), validRemoveInput())

	if err != nil || output.Removed || repo.calls != 1 {
		t.Fatalf("expected authorized no-op: %+v, %v", output, err)
	}
}

func TestRemoveRoleFromMemberCanceledContext(t *testing.T) {
	repo := &fakeMembershipRoleRemover{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewRemoveRoleFromMemberService(repo).
		Execute(ctx, validRemoveInput())

	if !errors.Is(err, context.Canceled) || repo.calls != 0 {
		t.Fatalf("unexpected canceled result: %v", err)
	}
}
