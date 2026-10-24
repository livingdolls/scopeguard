package api

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	systemhttp "github.com/livingdolls/scopeguard/apps/api/internal/modules/system/adapters/inbound/http"
	systempostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/system/adapters/outbound/postgres"
	systemapp "github.com/livingdolls/scopeguard/apps/api/internal/modules/system/application"
)

type systemModule struct {
	health    *systemhttp.Handler
	readiness *systemhttp.ReadinessHandler
}

func newSystem(pool *pgxpool.Pool, logger *slog.Logger) systemModule {
	probe := systempostgres.NewDatabaseProbe(pool)
	return systemModule{
		health:    systemhttp.NewHandler(systemapp.NewHealthService()),
		readiness: systemhttp.NewReadinessHandler(systemapp.NewReadinessService(probe), logger),
	}
}
