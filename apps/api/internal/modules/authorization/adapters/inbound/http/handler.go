package httpadapter

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
)

type Handler struct {
	listPermission application.ListPermissions
	logger         *slog.Logger
}

func NewHandler(listPermission application.ListPermissions, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		listPermission: listPermission,
		logger:         logger,
	}
}

type permissionResponse struct {
	Code        string `json:"code"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
	Description string `json:"description"`
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
