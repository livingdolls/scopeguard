package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type AssignRoleToMemberInput struct {
	ActorUserID    uuid.UUID
	OrganizationID uuid.UUID
	MembershipID   uuid.UUID
	RoleID         uuid.UUID
}

type AssignRoleToMemberOutput struct {
	Assigned bool
}

type AssignRoleToMember interface {
	Execute(ctx context.Context, input AssignRoleToMemberInput) (AssignRoleToMemberOutput, error)
}

type AssignRoleToMemberService struct {
	repository MembershipRoleRepository
}

var _ AssignRoleToMember = (*AssignRoleToMemberService)(nil)

func NewAssignRoleToMemberService(repository MembershipRoleRepository) *AssignRoleToMemberService {
	return &AssignRoleToMemberService{
		repository: repository,
	}
}

func (a AssignRoleToMemberService) Execute(ctx context.Context, input AssignRoleToMemberInput) (AssignRoleToMemberOutput, error) {
	if err := ctx.Err(); err != nil {
		return AssignRoleToMemberOutput{}, err
	}

	if input.ActorUserID == uuid.Nil || input.OrganizationID == uuid.Nil || input.MembershipID == uuid.Nil || input.RoleID == uuid.Nil {
		return AssignRoleToMemberOutput{}, ErrPermissionAssignmentForbidden
	}

	assigned, err := a.repository.Assign(ctx, input.ActorUserID, input.OrganizationID, input.MembershipID, input.RoleID)
	if err != nil {
		return AssignRoleToMemberOutput{}, fmt.Errorf("assign membership role: %w", err)
	}

	return AssignRoleToMemberOutput{Assigned: assigned}, nil
}
