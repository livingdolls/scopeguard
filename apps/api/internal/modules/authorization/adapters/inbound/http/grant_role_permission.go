package httpadapter

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
	"github.com/livingdolls/scopeguard/apps/api/internal/platform/authctx"
)

type grantRolePermissionRequest struct {
	PermissionCode string `json:"permission_code"`
}

type grantRolePermissionResponse struct {
	RoleID         uuid.UUID `json:"role_id"`
	PermissionCode string    `json:"permission_code"`
	Added          bool      `json:"added"`
}

func (h *Handler) GrantRolePermission(c *gin.Context) {
	c.Header("Cache-Control", "no-store")

	actorUserID, authenticated := authctx.UserIDFromContext(c.Request.Context())

	if !authenticated || actorUserID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authenticated required"})
		return
	}

	organizationID, err := uuid.Parse(c.Param("organization_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization_id"})
		return
	}

	roleID, err := uuid.Parse(c.Param("role_id"))
	if err != nil || roleID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role_id"})
		return
	}

	if c.ContentType() != "application/json" {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"error": "content-type must be application/json"})
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAuthorizationJSONBodySize)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()

	var request grantRolePermissionRequest

	if err := decoder.Decode(&request); err != nil {
		writeAuthorizationDecodeError(c, err)
		return
	}

	// reject trailing json values
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = errors.New("multiple JSON values")
		}

		writeAuthorizationDecodeError(c, err)
		return
	}

	output, err := h.grantRolePermission.Execute(
		c.Request.Context(),
		application.GrantRolePermissionInput{
			ActorUserID:    actorUserID,
			OrganizationID: organizationID,
			RoleID:         roleID,
			PermissionCode: request.PermissionCode,
		},
	)

	if err != nil {
		h.writeGrantRolePermissionError(c, err)
		return
	}

	status := http.StatusOK

	if output.Added {
		status = http.StatusCreated
	}

	c.JSON(status, grantRolePermissionResponse{
		RoleID:         roleID,
		PermissionCode: request.PermissionCode,
		Added:          output.Added,
	})
}

func (h *Handler) writeGrantRolePermissionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidPermissionCode):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid permission code"})

	case errors.Is(err, application.ErrPermissionAssignmentForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})

	case errors.Is(err, application.ErrTargetRoleNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "role not found"})

	case errors.Is(err, application.ErrPermissionNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "permission not found"})

	case errors.Is(err, application.ErrRoleOrganizationNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})

	case errors.Is(err, application.ErrSystemRoleImmutable):
		c.JSON(http.StatusConflict, gin.H{"error": "system role immutable"})

	default:
		h.logger.Error("grant role permission failed", "error", err)

		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "permission unavailable"})
	}
}
