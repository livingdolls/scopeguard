package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type RemoveRoleFromMemberInput struct {
	ActorUserID    uuid.UUID
	OrganizationID uuid.UUID
	MembershipID   uuid.UUID
	RoleID         uuid.UUID
}

type RemoveRoleFromMemberOutput struct {
	Removed bool
}

type RemoveRoleFromMember interface {
	Execute(ctx context.Context, input RemoveRoleFromMemberInput) (RemoveRoleFromMemberOutput, error)
}

type RemoveRoleFromMemberService struct {
	repository MembershipRoleRemover
}

var _ RemoveRoleFromMember = (*RemoveRoleFromMemberService)(nil)

func NewRemoveRoleFromMemberService(repository MembershipRoleRemover) *RemoveRoleFromMemberService {
	return &RemoveRoleFromMemberService{
		repository: repository,
	}
}

func (r *RemoveRoleFromMemberService) Execute(ctx context.Context, input RemoveRoleFromMemberInput) (RemoveRoleFromMemberOutput, error) {
	if err := ctx.Err(); err != nil {
		return RemoveRoleFromMemberOutput{}, err
	}

	if input.ActorUserID == uuid.Nil || input.OrganizationID == uuid.Nil || input.MembershipID == uuid.Nil || input.RoleID == uuid.Nil {
		return RemoveRoleFromMemberOutput{}, ErrPermissionAssignmentForbidden
	}

	removed, err := r.repository.Remove(ctx, input.ActorUserID, input.OrganizationID, input.MembershipID, input.RoleID)
	if err != nil {
		return RemoveRoleFromMemberOutput{}, fmt.Errorf("remove membership role: %w", err)
	}

	return RemoveRoleFromMemberOutput{Removed: removed}, nil
}
