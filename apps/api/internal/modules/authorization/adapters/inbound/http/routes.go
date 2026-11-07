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

	protected.GET("/organizations/:organization_id/permissions", requiredRead, h.ListPermissions)

	return nil
}
