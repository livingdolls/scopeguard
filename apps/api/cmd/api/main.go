package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	systemhttp "github.com/livingdolls/scopeguard/apps/api/internal/modules/system/adapters/inbound/http"
	systempostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/system/adapters/outbound/postgres"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/system/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/platform/config"
	"github.com/livingdolls/scopeguard/apps/api/internal/platform/database"
	"github.com/livingdolls/scopeguard/apps/api/internal/platform/httpapi"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// load .env if present
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.Error("failed to load .env", "error", err)
		os.Exit(1)
	}

	if err := run(logger); err != nil {
		logger.Error("application startup failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	// load config
	cfg, err := config.Load()

	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// handle termination signal

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)

	defer stop()

	// initialize Postgres Pool
	dbCtx, cancel := context.WithTimeout(ctx, 10*time.Second)

	pool, err := database.NewPool(dbCtx, cfg.DatabaseURL, cfg.DBMaxConnections)
	cancel()

	if err != nil {
		return fmt.Errorf("initialize database pool: %w", err)
	}
	defer pool.Close()

	logger.Info("PostgresSQL connected")

	// initalize outbound adapters
	databaseProbe := systempostgres.NewDatabaseProbe(pool)

	//initialize application service
	healthService := application.NewHealthService()
	readinessService := application.NewReadinessService(databaseProbe)

	//initialize inbound adapter
	healthHandler := systemhttp.NewHandler(healthService)
	readinessHandler := systemhttp.NewReadinessHandler(readinessService, logger)

	//register http routes
	router := httpapi.NewRouter()
	healthHandler.RegisterRoutes(router)
	readinessHandler.RegisterRoutes(router)

	//configure http server
	server := &http.Server{
		Handler:           router,
		Addr:              ":8080",
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErr := make(chan error, 1)

	go func() {
		serverErr <- server.ListenAndServe()
	}()

	logger.Info("Server started", "addr", server.Addr)

	// wait for termination or server failure
	select {
	case <-ctx.Done():
		logger.Info("Shutdown signal received")

	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("HTTP server error: %w", err)
		}

		return nil
	}

	// gracefully shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		if closeErr := server.Close(); closeErr != nil {
			return fmt.Errorf("force shutdown failed: %w", closeErr)
		}

		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	logger.Info("HTTP server stopped")
	return nil
}
