package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
)

type GrantRolePermissionInput struct {
	ActorUserID    uuid.UUID
	OrganizationID uuid.UUID
	RoleID         uuid.UUID
	PermissionCode string
}

type GrantRolePermissionOutput struct {
	Added bool
}

type GrantRolePermission interface {
	Execute(ctx context.Context, input GrantRolePermissionInput) (GrantRolePermissionOutput, error)
}

type GrantRolePermissionService struct {
	repository RolePermissionRepository
}

var _ GrantRolePermission = (*GrantRolePermissionService)(nil)

func NewGrantRolePermissionService(repository RolePermissionRepository) *GrantRolePermissionService {
	return &GrantRolePermissionService{
		repository: repository,
	}
}

func (g *GrantRolePermissionService) Execute(ctx context.Context, input GrantRolePermissionInput) (GrantRolePermissionOutput, error) {
	if err := ctx.Err(); err != nil {
		return GrantRolePermissionOutput{}, err
	}

	if input.ActorUserID == uuid.Nil || input.OrganizationID == uuid.Nil || input.RoleID == uuid.Nil {
		return GrantRolePermissionOutput{}, ErrPermissionAssignmentForbidden
	}

	permission, err := domain.ParsePermission(input.PermissionCode)
	if err != nil {
		return GrantRolePermissionOutput{}, fmt.Errorf("validate permission code: %w", err)
	}

	added, err := g.repository.Grant(ctx, input.ActorUserID, input.OrganizationID, input.RoleID, permission.Code())
	if err != nil {
		return GrantRolePermissionOutput{}, fmt.Errorf("grant role permission: %w", err)
	}

	return GrantRolePermissionOutput{
		Added: added,
	}, nil
}
