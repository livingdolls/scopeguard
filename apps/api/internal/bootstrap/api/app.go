package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/livingdolls/scopeguard/apps/api/internal/platform/database"
	"github.com/livingdolls/scopeguard/apps/api/internal/platform/httpapi"
)

type App struct {
	pool   *pgxpool.Pool
	server *http.Server
	logger *slog.Logger
}

func New(ctx context.Context, cfg Config, logger *slog.Logger) (*App, error) {
	if logger == nil {
		logger = slog.Default()
	}

	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	pool, err := database.NewPool(dbCtx, cfg.DatabaseURL, cfg.DBMaxConnections)
	cancel()
	if err != nil {
		return nil, fmt.Errorf("initialize database pool: %w", err)
	}

	router, err := NewRouter(pool, cfg, logger)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("initialize API router: %w", err)
	}

	logger.Info("PostgreSQL connected")
	return &App{
		pool:   pool,
		server: httpapi.NewServer(cfg.HTTPAddr, router),
		logger: logger,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	return httpapi.RunServer(ctx, a.server, a.logger)
}

// Close releases the database pool after Run has stopped serving requests.
func (a *App) Close() {
	a.pool.Close()
}
