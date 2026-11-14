package httpadapter

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/platform/authctx"
)

type membershipRolesResponse struct {
	MembershipID uuid.UUID      `json:"membership_id"`
	Roles        []roleResponse `json:"roles"`
}

func (h *Handler) ListMembershipRoles(c *gin.Context) {
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

	membershipID, err := uuid.Parse(c.Param("membership_id"))
	if err != nil || membershipID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid membership_id"})
		return
	}

	roles, err := h.listMembershipRoles.Execute(c.Request.Context(), organizationID, membershipID)
	if err != nil {
		switch {
		case errors.Is(err, application.ErrMembershipNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "membership not found"})

		case errors.Is(err, application.ErrRoleOrganizationNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})

		default:
			h.logger.Error("list membership failed", "error", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "membership roles unavailable"})
		}
		return
	}

	responseRoles := make([]roleResponse, 0, len(roles))

	for _, role := range roles {
		responseRoles = append(responseRoles, roleResponse{
			ID:             role.ID,
			OrganizationID: role.OrganizationID,
			Name:           role.Name,
			Slug:           role.Slug,
			IsSystem:       role.IsSystem,
			CreatedAt:      role.CreatedAt,
			UpdatedAt:      role.UpdatedAt,
		})
	}

	c.JSON(http.StatusOK, membershipRolesResponse{
		MembershipID: membershipID,
		Roles:        responseRoles,
	})
}
