package httpadapter

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/platform/authctx"
)

type rolePermissionsResponse struct {
	RoleID      uuid.UUID `json:"role_id"`
	Permissions []string  `json:"permissions"`
}

func (h *Handler) ListRolePermissions(c *gin.Context) {
	c.Header("Cache-Control", "no-store")

	actorID, authenticated := authctx.UserIDFromContext(c.Request.Context())

	if !authenticated || actorID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authenticated required"})
		return
	}

	organizationID, err := uuid.Parse(c.Param("organization_id"))
	if err != nil || organizationID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization_id"})
		return
	}

	roleID, err := uuid.Parse(c.Param("role_id"))
	if err != nil || roleID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role_id"})
		return
	}

	permissions, err := h.listRolePermissions.Execute(
		c.Request.Context(),
		organizationID,
		roleID,
	)

	if err != nil {
		switch {
		case errors.Is(err, application.ErrTargetRoleNotFound):
			c.JSON(
				http.StatusNotFound,
				gin.H{"error": "role not found"},
			)
		case errors.Is(err, application.ErrRoleOrganizationNotFound):
			c.JSON(
				http.StatusNotFound,
				gin.H{"error": "organization not found"},
			)
		default:
			h.logger.Error(
				"list role permissions failed",
				"error", err,
			)
			c.JSON(
				http.StatusServiceUnavailable,
				gin.H{"error": "role permissions unavailable"},
			)
		}
		return
	}

	c.JSON(http.StatusOK, rolePermissionsResponse{
		RoleID:      roleID,
		Permissions: permissions,
	})
}
