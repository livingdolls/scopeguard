package application

import (
	"context"

	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/relationship/domain"
)

type AccessFactsReader interface {
	ReadForActor(ctx context.Context, actorUserID uuid.UUID, organizationID uuid.UUID, resourceID uuid.UUID) (domain.AccessFacts, error)
}
