package application

import (
	"context"

	"github.com/livingdolls/scopeguard/apps/api/internal/modules/organization/domain"
)

// OrganizationRepository defines the persistence
// operations required by application use case/service
type OrganizationRepository interface {
	Create(ctx context.Context, org *domain.Organization) error
}
