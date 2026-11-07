package httpadapter

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/platform/authctx"
)

type assignRoleToMemberResponse struct {
	MembershipID uuid.UUID `json:"membership_id"`
	RoleID       uuid.UUID `json:"role_id"`
	Assigned     bool      `json:"assigned"`
}

func (h *Handler) AssignRoleToMember(c *gin.Context) {
	c.Header("Cache-Control", "no-store")

	actorID, authenticated := authctx.UserIDFromContext(c.Request.Context())

	if !authenticated || actorID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
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

	roleID, err := uuid.Parse(c.Param("role_id"))
	if err != nil || roleID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role_id"})
		return
	}

	output, err := h.assignRoleToMember.Execute(
		c.Request.Context(),
		application.AssignRoleToMemberInput{
			ActorUserID:    actorID,
			OrganizationID: organizationID,
			MembershipID:   membershipID,
			RoleID:         roleID,
		},
	)

	if err != nil {
		h.writeAssignRoleToMemberError(c, err)
		return
	}

	status := http.StatusOK
	if output.Assigned {
		status = http.StatusCreated
	}

	c.JSON(status, assignRoleToMemberResponse{
		MembershipID: membershipID,
		RoleID:       roleID,
		Assigned:     output.Assigned,
	})
}

func (h *Handler) writeAssignRoleToMemberError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, application.ErrPermissionAssignmentForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})

	case errors.Is(err, application.ErrRoleOrganizationNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "organization not found"})

	case errors.Is(err, application.ErrTargetRoleNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "role not found"})

	case errors.Is(err, application.ErrMembershipNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "membership not found"})

	case errors.Is(err, application.ErrOwnerRoleAssignmentForbidden):
		c.JSON(http.StatusConflict, gin.H{"error": "owner role assignment forbidden"})

	default:
		h.logger.Error("assign role to membership failed", "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "role assignment unavailable"})
	}
}
