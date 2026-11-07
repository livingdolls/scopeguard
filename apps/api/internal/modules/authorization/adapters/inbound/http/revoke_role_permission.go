package httpadapter

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
	"github.com/livingdolls/scopeguard/apps/api/internal/platform/authctx"
)

type revokeRolePermissionResponse struct {
	RoleID         uuid.UUID `json:"role_id"`
	PermissionCode string    `json:"permission_code"`
	Removed        bool      `json:"removed"`
}

func (h *Handler) RevokeRolePermission(c *gin.Context) {
	c.Header("Cache-Control", "no-store")

	actorUserID, authenticated := authctx.UserIDFromContext(c.Request.Context())

	if !authenticated || actorUserID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	organizationID, err := uuid.Parse(c.Param("organization_id"))
	if err != nil || organizationID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organization_id is invalid"})
		return
	}

	roleID, err := uuid.Parse(c.Param("role_id"))
	if err != nil || roleID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role_id is invalid"})
		return
	}

	permissionCode := c.Param("permission_code")

	// Domain validation happens again in the application service
	if _, err := domain.ParsePermission(permissionCode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "permission_code is invalid"})
		return
	}

	output, err := h.revokeRolePermission.Execute(
		c.Request.Context(),
		application.RevokeRolePermissionInput{
			ActorUserID:    actorUserID,
			OrganizationID: organizationID,
			RoleID:         roleID,
			PermissionCode: permissionCode,
		},
	)

	if err != nil {
		h.writeRevokeRolePermissionError(c, err)
		return
	}

	c.JSON(http.StatusOK, revokeRolePermissionResponse{
		RoleID:         roleID,
		PermissionCode: permissionCode,
		Removed:        output.Removed,
	})
}

func (h *Handler) writeRevokeRolePermissionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidPermissionCode):
		c.JSON(http.StatusBadRequest, gin.H{"error": "permission code is invalid"})
	case errors.Is(err, application.ErrPermissionAssignmentForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
	case errors.Is(err, application.ErrTargetRoleNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "target role not found"})
	case errors.Is(err, application.ErrPermissionNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "permission not found"})
	case errors.Is(err, application.ErrRoleOrganizationNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})
	case errors.Is(err, application.ErrSystemRoleImmutable):
		c.JSON(http.StatusConflict, gin.H{"error": "system role is immutable"})

	default:
		h.logger.Error("revoke permission failed", "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "permission revoke unavailable"})
	}
}
