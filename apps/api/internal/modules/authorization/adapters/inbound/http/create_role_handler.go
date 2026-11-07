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

const maxCreateRoleBodySize = 16 << 10 // 16Kb

type createRoleRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (h *Handler) CreateRole(c *gin.Context) {
	c.Header("Cache-Control", "no-store")

	actorUserID, authenticated := authctx.UserIDFromContext(c.Request.Context())

	if !authenticated || actorUserID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "authenticated required",
		})
	}

	organizationID, err := uuid.Parse(c.Param("organization_id"))
	if err != nil || organizationID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "organization_id required",
		})

		return
	}

	if c.ContentType() != "application/json" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Content-Type must be application/json",
		})

		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCreateRoleBodySize)

	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields() // tolak properti  yang tidak di definisikan di createRoleRequest

	var request createRoleRequest

	if err := decoder.Decode(&request); err != nil {
		writeCreateRoleDecodeError(c, err)
		return
	}

	// reject multiple values JSON in one request body
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = errors.New("multiple json values")
		}

		writeCreateRoleDecodeError(c, err)
		return
	}

	output, err := h.createRole.Execute(
		c.Request.Context(),
		application.CreateRoleInput{
			ActorUserID:    actorUserID,
			OrganizationID: organizationID,
			Name:           request.Name,
			Slug:           request.Slug,
		},
	)

	if err != nil {
		h.writeCreateRoleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, roleResponse{
		ID:             output.ID,
		OrganizationID: output.OrganizationID,
		Name:           output.Name,
		Slug:           output.Slug,
		IsSystem:       output.IsSystem,
		CreatedAt:      output.CreatedAt,
		UpdatedAt:      output.UpdatedAt,
	})
}

func writeCreateRoleDecodeError(c *gin.Context, err error) {
	var maxBytesErr *http.MaxBytesError

	if errors.As(err, &maxBytesErr) {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": "request body too large",
		})

		return
	}

	c.JSON(http.StatusBadRequest, gin.H{
		"error": "invalid JSON request",
	})
}

func (h *Handler) writeCreateRoleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidRoleName),
		errors.Is(err, domain.ErrInvalidRoleSlug),
		errors.Is(err, domain.ErrInvalidRoleSystemFlag),
		errors.Is(err, domain.ErrInvalidRoleOrganizationID):
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid role data",
		})

	case errors.Is(err, application.ErrRoleSlugAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{
			"error": "role slug already exists",
		})

	case errors.Is(err, application.ErrPermissionAssignmentForbidden):
		c.JSON(http.StatusForbidden, gin.H{
			"error": "access denied",
		})

	case errors.Is(err, application.ErrRoleOrganizationNotFound):
		c.JSON(http.StatusNotFound, gin.H{
			"error": "role organization not found",
		})

	default:
		h.logger.Error("create organization role failed", "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "role creation unavailable",
		})
	}
}
