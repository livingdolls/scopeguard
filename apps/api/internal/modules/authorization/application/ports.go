package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
)

type RoleRepository interface {
	Insert(ctx context.Context, role *domain.Role) error
}

type RolePermissionRepository interface {
	Grant(ctx context.Context, actorUserID uuid.UUID, organizationID uuid.UUID, roleID uuid.UUID, permissionCode string) (bool, error)
}
