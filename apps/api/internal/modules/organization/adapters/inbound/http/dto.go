package httpadapter

import (
	"time"

	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/organization/application"
)

type createOrganizationRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type organizationResponse struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func toOrganizationResponse(output application.CreateOrganizationOutput) organizationResponse {
	return organizationResponse{
		ID:        output.ID,
		Name:      output.Name,
		Slug:      output.Slug,
		CreatedAt: output.CreatedAt,
		UpdatedAt: output.UpdatedAt,
	}
}
