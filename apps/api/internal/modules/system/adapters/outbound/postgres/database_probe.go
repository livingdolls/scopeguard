package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/system/application"
)

type DatabaseProbe struct {
	pool *pgxpool.Pool
}

var _ application.DatabaseProbe = (*DatabaseProbe)(nil)

func NewDatabaseProbe(pool *pgxpool.Pool) *DatabaseProbe {
	return &DatabaseProbe{
		pool: pool,
	}
}

func (p *DatabaseProbe) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}
