package httpadapter

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/organization/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/organization/domain"
	"github.com/livingdolls/scopeguard/apps/api/internal/platform/authctx"
)

const maxRequestBodySize = 16 << 10 // 16kb

type Handler struct {
	createOrganization application.CreateOrganization
	logger             *slog.Logger
}

func NewHandler(createOrganization application.CreateOrganization, logger *slog.Logger) *Handler {
	return &Handler{
		createOrganization: createOrganization,
		logger:             logger,
	}
}

func (h *Handler) CreateOrganization(c *gin.Context) {
	ownerID, authenticated := authctx.UserIDFromContext(c.Request.Context())

	if !authenticated {
		c.JSON(http.StatusUnauthorized, errorResponse{
			Error: "authentication required",
		})

		return
	}

	// accept JSON request only
	if c.ContentType() != "application/json" {
		c.JSON(
			http.StatusUnsupportedMediaType,
			errorResponse{Error: "content type is not application/json"},
		)
		return
	}

	// limit request body size
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxRequestBodySize)

	// decode JSON
	var request createOrganizationRequest

	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		h.writeDecodeError(c, err)
		return
	}

	// reject multiple json values in a single body
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = errors.New("multiple JSON values")
		}

		h.writeDecodeError(c, err)
		return
	}

	// map http request to application input
	input := application.CreateOrganizationInput{
		Name:        request.Name,
		Slug:        request.Slug,
		OwnerUserID: ownerID,
	}

	// execute application use case
	output, err := h.createOrganization.Execute(c.Request.Context(), input)

	if err != nil {
		h.writeApplicationError(c, err)
		return
	}

	// map application output to Http response
	c.JSON(http.StatusCreated, toOrganizationResponse(output))
}

func (h *Handler) writeDecodeError(c *gin.Context, err error) {

	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		c.JSON(
			http.StatusRequestEntityTooLarge,
			errorResponse{Error: "request body too large"},
		)
		return
	}

	c.JSON(
		http.StatusBadRequest,
		errorResponse{Error: "invalid JSON request"},
	)

}

func (h *Handler) writeApplicationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidOrganizationName):
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid organization name"})
	case errors.Is(err, domain.ErrInvalidOrganizationSlug):
		c.JSON(http.StatusBadRequest, errorResponse{Error: "invalid organization slug"})
	case errors.Is(err, application.ErrOrganizationSlugAlreadyExists):
		c.JSON(http.StatusConflict, errorResponse{Error: "organization already exists"})
	case errors.Is(err, application.ErrOwnerUserNotFound), errors.Is(err, application.ErrInvalidOwnerUserID):
		c.JSON(http.StatusUnauthorized, errorResponse{Error: "authentication required"})
	default:
		h.logger.Error("create organization failed", "error", err)
		c.JSON(http.StatusInternalServerError, errorResponse{Error: "internal server error"})
	}
}
