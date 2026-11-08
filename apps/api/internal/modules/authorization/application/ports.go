package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
)

type PermissionCatalogEntry struct {
	Code        string
	Resource    string
	Action      string
	Description string
}

type RoleListItem struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	Slug           string
	IsSystem       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type RoleRepository interface {
	Insert(ctx context.Context, actorUserID uuid.UUID, role *domain.Role) error
}

type RolePermissionRepository interface {
	Grant(ctx context.Context, actorUserID uuid.UUID, organizationID uuid.UUID, roleID uuid.UUID, permissionCode string) (bool, error)
}

type RolePermissionRevoker interface {
	Revoke(ctx context.Context, actorUserID uuid.UUID, organizationID uuid.UUID, roleID uuid.UUID, permissionCode string) (removed bool, err error)
}

type MembershipRoleRepository interface {
	Assign(ctx context.Context, actorUserID uuid.UUID, organizationID uuid.UUID, membershipID uuid.UUID, roleID uuid.UUID) (assigned bool, err error)
}

type MembershipRoleRemover interface {
	Remove(ctx context.Context, actorUserID uuid.UUID, organizationID uuid.UUID, membershipID uuid.UUID, roleID uuid.UUID) (removed bool, err error)
}

type PermissionReader interface {
	HasPermission(ctx context.Context, userID uuid.UUID, organizationID uuid.UUID, permissionCode string) (bool, error)
}

type PermissionCatalogReader interface {
	List(ctx context.Context) ([]PermissionCatalogEntry, error)
}

type RoleReader interface {
	ListByOrganization(ctx context.Context, organizationID uuid.UUID) ([]RoleListItem, error)
}

type RolePermissionReader interface {
	ListByRole(ctx context.Context, organizationID uuid.UUID, roleID uuid.UUID) ([]string, error)
}
