package httpadapter

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
)

type Handler struct {
	listPermission       application.ListPermissions
	listRoles            application.ListRoles
	createRole           application.CreateRole
	grantRolePermission  application.GrantRolePermission
	revokeRolePermission application.RevokeRolePermission
	assignRoleToMember   application.AssignRoleToMember
	removeRoleFromMember application.RemoveRoleFromMember
	listRolePermissions  application.ListRolePermissions
	listMembershipRoles  application.ListMembershipRoles
	logger               *slog.Logger
}

func NewHandler(
	listPermission application.ListPermissions,
	listRoles application.ListRoles,
	createRole application.CreateRole,
	grantRolePermission application.GrantRolePermission,
	revokeRolePermission application.RevokeRolePermission,
	assignRoleToMember application.AssignRoleToMember,
	removeRoleFromMember application.RemoveRoleFromMember,
	listRolePermissions application.ListRolePermissions,
	listMembershipRoles application.ListMembershipRoles,
	logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		listPermission:       listPermission,
		listRoles:            listRoles,
		createRole:           createRole,
		grantRolePermission:  grantRolePermission,
		revokeRolePermission: revokeRolePermission,
		assignRoleToMember:   assignRoleToMember,
		removeRoleFromMember: removeRoleFromMember,
		listRolePermissions:  listRolePermissions,
		listMembershipRoles:  listMembershipRoles,
		logger:               logger,
	}
}

type permissionResponse struct {
	Code        string `json:"code"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
	Description string `json:"description"`
}

type roleResponse struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	Name           string    `json:"name"`
	Slug           string    `json:"slug"`
	IsSystem       bool      `json:"is_system"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (h *Handler) ListPermissions(c *gin.Context) {
	permissions, err := h.listPermission.Execute(c.Request.Context())

	if err != nil {
		h.logger.Error("list permission failed", "error", err)

		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "permissions unavailable"})
		return
	}

	response := make([]permissionResponse, 0, len(permissions))

	for _, permission := range permissions {
		response = append(response, permissionResponse{
			Code:        permission.Code,
			Resource:    permission.Resource,
			Action:      permission.Action,
			Description: permission.Description,
		})
	}

	c.JSON(http.StatusOK, gin.H{"permissions": response})
}

func (h *Handler) ListRoles(c *gin.Context) {
	c.Header("Cache-Control", "no-store")

	organizationID, err := uuid.Parse(c.Param("organization_id"))
	if err != nil || organizationID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organization_id is invalid"})

		return
	}

	roles, err := h.listRoles.Execute(c.Request.Context(), organizationID)
	if err != nil {
		h.logger.Error("list organization roles failed", "error", err)

		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "roles unavailable"})
		return
	}

	response := make([]roleResponse, 0, len(roles))

	for _, role := range roles {
		response = append(response, roleResponse{
			ID:             role.ID,
			OrganizationID: role.OrganizationID,
			Name:           role.Name,
			Slug:           role.Slug,
			IsSystem:       role.IsSystem,
			CreatedAt:      role.CreatedAt,
			UpdatedAt:      role.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"roles": response})
}
