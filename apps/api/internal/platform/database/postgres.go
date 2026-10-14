package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(
	ctx context.Context,
	databaseURL string,
	maxConnections int32,
) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)

	if err != nil {
		return nil, errors.New("invalid DATABASE_URL")
	}

	if maxConnections < 1 {
		return nil, errors.New("MAX_CONNECTIONS must be greater than zero")
	}

	cfg.MaxConns = maxConnections

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create postgresSQL pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("ping postgresSQL pool: %w", err)
	}

	return pool, nil
}
