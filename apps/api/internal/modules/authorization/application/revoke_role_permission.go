package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
)

type RevokeRolePermissionInput struct {
	ActorUserID    uuid.UUID
	OrganizationID uuid.UUID
	RoleID         uuid.UUID
	PermissionCode string
}

type RevokeRolePermissionOutput struct {
	Removed bool
}

type RevokeRolePermission interface {
	Execute(ctx context.Context, input RevokeRolePermissionInput) (RevokeRolePermissionOutput, error)
}

type RevokeRolePermissionService struct {
	repository RolePermissionRevoker
}

var _ RevokeRolePermission = (*RevokeRolePermissionService)(nil)

func NewRevokeRolePermissionService(repository RolePermissionRevoker) *RevokeRolePermissionService {
	return &RevokeRolePermissionService{repository: repository}
}

func (r *RevokeRolePermissionService) Execute(ctx context.Context, input RevokeRolePermissionInput) (RevokeRolePermissionOutput, error) {
	if err := ctx.Err(); err != nil {
		return RevokeRolePermissionOutput{}, err
	}

	if input.ActorUserID == uuid.Nil || input.OrganizationID == uuid.Nil || input.RoleID == uuid.Nil {
		return RevokeRolePermissionOutput{}, ErrPermissionAssignmentForbidden
	}

	permission, err := domain.ParsePermission(input.PermissionCode)
	if err != nil {
		return RevokeRolePermissionOutput{}, fmt.Errorf("validate permission: %w", err)
	}

	removed, err := r.repository.Revoke(ctx, input.ActorUserID, input.OrganizationID, input.RoleID, permission.Code())
	if err != nil {
		return RevokeRolePermissionOutput{}, fmt.Errorf("revoke role permission: %w", err)
	}

	return RevokeRolePermissionOutput{Removed: removed}, nil
}
