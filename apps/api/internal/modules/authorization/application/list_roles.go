package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type ListRoles interface {
	Execute(ctx context.Context, organizationID uuid.UUID) ([]RoleListItem, error)
}

type listRoleService struct {
	reader RoleReader
}

var _ ListRoles = (*listRoleService)(nil)

func NewListRolesService(reader RoleReader) *listRoleService {
	return &listRoleService{
		reader: reader,
	}
}

func (l *listRoleService) Execute(ctx context.Context, organizationID uuid.UUID) ([]RoleListItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if organizationID == uuid.Nil {
		return nil, ErrRoleOrganizationNotFound
	}

	roles, err := l.reader.ListByOrganization(ctx, organizationID)

	if err != nil {
		return nil, fmt.Errorf("list organization roles: %w", err)
	}

	return roles, nil
}
