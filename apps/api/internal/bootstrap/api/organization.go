package api

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	organizationhttp "github.com/livingdolls/scopeguard/apps/api/internal/modules/organization/adapters/inbound/http"
	organizationpostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/organization/adapters/outbound/postgres"
	organizationapp "github.com/livingdolls/scopeguard/apps/api/internal/modules/organization/application"
)

func newOrganization(pool *pgxpool.Pool, logger *slog.Logger) *organizationhttp.Handler {
	repository := organizationpostgres.NewOrganizationRepository(pool)
	service := organizationapp.NewCreateOrganizationService(repository)
	return organizationhttp.NewHandler(service, logger)
}
