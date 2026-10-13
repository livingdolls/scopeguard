package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	systemhttp "github.com/livingdolls/scopeguard/apps/api/internal/modules/system/adapters/inbound/http"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/system/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/platform/httpapi"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("application startup failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	healthService := application.NewHealthService()
	healthHandler := systemhttp.NewHandler(healthService)

	router := httpapi.NewRouter()
	healthHandler.RegisterRoutes(router)

	server := &http.Server{
		Handler:           router,
		Addr:              ":8080",
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)

	defer stop()

	serverErr := make(chan error, 1)

	go func() {
		serverErr <- server.ListenAndServe()
	}()

	logger.Info("Server started", "addr", server.Addr)

	select {
	case <-ctx.Done():
		logger.Info("Shutdown signal received")

	case err := <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("HTTP server error: %w", err)
		}

		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	logger.Info("Shutdown complete")
	return nil
}
