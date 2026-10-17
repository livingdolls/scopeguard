package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/organization/domain"
)

// Application input model
type CreateOrganizationInput struct {
	Name        string
	Slug        string
	OwnerUserID uuid.UUID
}

// Application output model
type CreateOrganizationOutput struct {
	ID        uuid.UUID
	Name      string
	Slug      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// inbound port
type CreateOrganization interface {
	Execute(ctx context.Context, input CreateOrganizationInput) (CreateOrganizationOutput, error)
}

// application service
type CreateOrganizationService struct {
	repository OrganizationRepository
}

var _ CreateOrganization = (*CreateOrganizationService)(nil)

func NewCreateOrganizationService(repository OrganizationRepository) *CreateOrganizationService {
	return &CreateOrganizationService{
		repository: repository,
	}
}

func (s CreateOrganizationService) Execute(ctx context.Context, input CreateOrganizationInput) (CreateOrganizationOutput, error) {
	if err := ctx.Err(); err != nil {
		return CreateOrganizationOutput{}, err
	}

	if input.OwnerUserID == uuid.Nil {
		return CreateOrganizationOutput{}, ErrInvalidOwnerUserID
	}

	// generate ID and timestamp
	id := uuid.New()
	now := time.Now()

	// create domain entiry
	org, err := domain.NewOrganization(
		id,
		input.Name,
		input.Slug,
		now,
	)

	if err != nil {
		return CreateOrganizationOutput{}, fmt.Errorf("create organization entity: %w", err)
	}

	// persist through outbound port
	if err := s.repository.CreateWithOwner(ctx, org, input.OwnerUserID); err != nil {
		return CreateOrganizationOutput{}, fmt.Errorf("persist organization: %w", err)
	}

	// return application output
	return CreateOrganizationOutput{
		ID:        org.ID(),
		Name:      org.Name(),
		Slug:      org.Slug(),
		CreatedAt: org.CreatedAt(),
		UpdatedAt: org.UpdatedAt(),
	}, nil
}
