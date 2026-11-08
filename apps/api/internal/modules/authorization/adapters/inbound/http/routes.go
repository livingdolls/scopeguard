package httpadapter

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
)

func (h *Handler) RegisterRoutes(protected *gin.RouterGroup, guard *PermissionGuard) error {
	requiredRead, err := guard.RequirePermission(domain.PermissionPermissionRead)

	if err != nil {
		return fmt.Errorf("register permission catalog router: %w", err)
	}

	requiredRoleRead, err := guard.RequirePermission(domain.PermissionRoleRead)
	if err != nil {
		return fmt.Errorf("register role listing route: %w", err)
	}

	requiredRoleCreate, err := guard.RequirePermission(domain.PermissionRoleCreate)
	if err != nil {
		return fmt.Errorf("register role creating route: %w", err)
	}

	requiredRoleUpdate, err := guard.RequirePermission(domain.PermissionRoleUpdate)
	if err != nil {
		return fmt.Errorf("register role permission grant route: %w", err)
	}

	requiredMemberRoleAssign, err := guard.RequirePermission(domain.PermissionMemberRoleAssign)
	if err != nil {
		return fmt.Errorf("register member role assign route: %w", err)
	}

	protected.GET("/organizations/:organization_id/permissions", requiredRead, h.ListPermissions)
	protected.GET("/organizations/:organization_id/roles", requiredRoleRead, h.ListRoles)
	protected.POST("/organizations/:organization_id/roles", requiredRoleCreate, h.CreateRole)
	protected.POST("/organizations/:organization_id/roles/:role_id/permissions", requiredRoleUpdate, h.GrantRolePermission)
	protected.DELETE("/organizations/:organization_id/roles/:role_id/permissions/:permission_code", requiredRoleUpdate, h.RevokeRolePermission)
	protected.GET("/organizations/:organization_id/roles/:role_id/permissions", requiredRead, h.ListRolePermissions)
	protected.POST("/organizations/:organization_id/memberships/:membership_id/roles/:role_id", requiredMemberRoleAssign, h.AssignRoleToMember)
	protected.DELETE("/organizations/:organization_id/memberships/:membership_id/roles/:role_id", requiredMemberRoleAssign, h.RemoveRoleFromMember)

	return nil
}
