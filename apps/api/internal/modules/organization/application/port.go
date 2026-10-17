package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/organization/domain"
)

// OrganizationRepository defines the persistence
// operations required by application use case/service
type OrganizationRepository interface {
	CreateWithOwner(ctx context.Context, org *domain.Organization, ownerUserID uuid.UUID) error
}
