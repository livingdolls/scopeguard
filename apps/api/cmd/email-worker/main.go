package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	encryptionadapter "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/adapters/outbound/encryption"
	identitypostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/adapters/outbound/postgres"
	smtpadapter "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/adapters/outbound/smtp"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"

	"github.com/livingdolls/scopeguard/apps/api/internal/platform/database"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.Error("failed to load .env file")
		os.Exit(1)
	}

	if err := run(logger); err != nil {
		logger.Error("email worker stopped with error", "error", err)
		os.Exit(1)
	}

}

func run(logger *slog.Logger) error {
	cfg, err := loadWorkerConfig()
	if err != nil {
		return fmt.Errorf("failed to load worker config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	defer stop()

	dbCtx, cancelDB := context.WithTimeout(ctx, 10*time.Second)

	pool, err := database.NewPool(dbCtx, cfg.DatabaseURL, cfg.DBMaxConnections)
	cancelDB()

	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	opener, err := encryptionadapter.NewAESGCM(cfg.EncryptionKey, cfg.EncryptionKeyID)
	if err != nil {
		return fmt.Errorf("failed to create opener: %w", err)
	}

	sender, err := smtpadapter.NewSender(
		smtpadapter.Config{
			Host:            cfg.SMTPHost,
			Port:            cfg.SMTPPort,
			TLSMode:         cfg.SMTPTLSMode,
			Username:        cfg.SMTPUsername,
			Password:        cfg.SMTPPassword,
			From:            cfg.SMTPFrom,
			VerificationURL: cfg.VerificationURL,
		},
	)

	if err != nil {
		return fmt.Errorf("failed to create SMTP client: %w", err)
	}

	jobs := identitypostgres.NewEmailOutboxJobRepository(pool)

	worker := application.NewEmailWorker(jobs, opener, sender, logger)

	logger.Info("starting email worker", "concurrency", cfg.Concurrency)

	if err := worker.Run(ctx, cfg.Concurrency); err != nil {
		return fmt.Errorf("failed to start email worker: %w", err)
	}

	logger.Info("email worker has been stopped")
	return nil
}
