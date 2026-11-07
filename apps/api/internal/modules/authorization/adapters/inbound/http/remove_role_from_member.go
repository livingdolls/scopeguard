package httpadapter

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/platform/authctx"
)

type removeRoleFromMemberResponse struct {
	MembershipID uuid.UUID `json:"membership_id"`
	RoleID       uuid.UUID `json:"role_id"`
	Removed      bool      `json:"removed"`
}

func (h *Handler) RemoveRoleFromMember(c *gin.Context) {
	c.Header("Cache-Control", "no-cache")

	actorUserID, authenticated := authctx.UserIDFromContext(c.Request.Context())

	if !authenticated || actorUserID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "access denied"})
		return
	}

	organizationID, err := uuid.Parse(c.Param("organization_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization_id"})
		return
	}

	membershipID, err := uuid.Parse(c.Param("membership_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid membership_id"})
		return
	}

	roleID, err := uuid.Parse(c.Param("role_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role_id"})
		return
	}

	output, err := h.removeRoleFromMember.Execute(
		context.Background(),
		application.RemoveRoleFromMemberInput{
			ActorUserID:    actorUserID,
			OrganizationID: organizationID,
			MembershipID:   membershipID,
			RoleID:         roleID,
		})

	if err != nil {
		h.writeRemoveRoleFromMemberError(c, err)
		return
	}

	c.JSON(http.StatusOK, removeRoleFromMemberResponse{
		MembershipID: membershipID,
		RoleID:       roleID,
		Removed:      output.Removed,
	})

}

func (h *Handler) writeRemoveRoleFromMemberError(
	c *gin.Context,
	err error,
) {
	switch {
	case errors.Is(err, application.ErrPermissionAssignmentForbidden):
		c.JSON(
			http.StatusForbidden,
			gin.H{"error": "access denied"},
		)

	case errors.Is(err, application.ErrRoleOrganizationNotFound):
		c.JSON(
			http.StatusNotFound,
			gin.H{"error": "organization not found"},
		)

	case errors.Is(err, application.ErrMembershipNotFound):
		c.JSON(
			http.StatusNotFound,
			gin.H{"error": "membership not found"},
		)

	case errors.Is(err, application.ErrTargetRoleNotFound):
		c.JSON(
			http.StatusNotFound,
			gin.H{"error": "role not found"},
		)

	case errors.Is(err, application.ErrOwnerRoleRemovalForbidden):
		c.JSON(
			http.StatusConflict,
			gin.H{"error": "owner role removal forbidden"},
		)

	case errors.Is(err, application.ErrLastMembershipRole):
		c.JSON(
			http.StatusConflict,
			gin.H{"error": "cannot remove the last membership role"},
		)

	default:
		h.logger.Error(
			"remove role from membership failed",
			"error", err,
		)

		c.JSON(
			http.StatusServiceUnavailable,
			gin.H{"error": "membership role removal unavailable"},
		)
	}
}
