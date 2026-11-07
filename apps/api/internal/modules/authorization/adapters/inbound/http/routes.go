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

	protected.GET("/organizations/:organization_id/permissions", requiredRead, h.ListPermissions)
	protected.GET("/organizations/:organization_id/roles", requiredRoleRead, h.ListRoles)
	protected.POST("/organizations/:organization_id/roles", requiredRoleCreate, h.CreateRole)

	return nil
}
