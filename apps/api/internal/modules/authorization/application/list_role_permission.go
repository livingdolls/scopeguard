package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type ListRolePermissions interface {
	Execute(ctx context.Context, organizationID uuid.UUID, roleID uuid.UUID) ([]string, error)
}

type listRolePermissionsService struct {
	reader RolePermissionReader
}

var _ ListRolePermissions = (*listRolePermissionsService)(nil)

func NewListRolePermissionsService(reader RolePermissionReader) *listRolePermissionsService {
	return &listRolePermissionsService{
		reader: reader,
	}
}

func (s *listRolePermissionsService) Execute(ctx context.Context, organizationID uuid.UUID, roleID uuid.UUID) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if organizationID == uuid.Nil {
		return nil, ErrRoleOrganizationNotFound
	}

	if roleID == uuid.Nil {
		return nil, ErrTargetRoleNotFound
	}

	permissions, err := s.reader.ListByRole(ctx, organizationID, roleID)
	if err != nil {
		return nil, fmt.Errorf("list role permissions: %w", err)
	}

	if permissions == nil {
		return []string{}, nil
	}

	return permissions, nil
}
