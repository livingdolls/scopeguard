package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type ListMembershipRoles interface {
	Execute(ctx context.Context, organizationID uuid.UUID, membershipID uuid.UUID) ([]RoleListItem, error)
}

type listMembershipRolesService struct {
	reader MembershipRolesReader
}

var _ ListMembershipRoles = (*listMembershipRolesService)(nil)

func NewListMembershipRolesService(reader MembershipRolesReader) *listMembershipRolesService {
	return &listMembershipRolesService{
		reader: reader,
	}
}

func (l *listMembershipRolesService) Execute(ctx context.Context, organizationID uuid.UUID, membershipID uuid.UUID) ([]RoleListItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if organizationID == uuid.Nil {
		return nil, ErrRoleOrganizationNotFound
	}

	if membershipID == uuid.Nil {
		return nil, ErrMembershipNotFound
	}

	roles, err := l.reader.ListByMembership(ctx, organizationID, membershipID)
	if err != nil {
		return nil, fmt.Errorf("list membership roles: %w", err)
	}

	if roles == nil {
		return []RoleListItem{}, nil
	}

	return roles, nil
}
