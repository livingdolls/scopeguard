package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
)

type PermissionCatalogRepository struct {
	pool *pgxpool.Pool
}

var _ application.PermissionCatalogReader = (*PermissionCatalogRepository)(nil)

func NewPermissionCatalogRepository(pool *pgxpool.Pool) *PermissionCatalogRepository {
	return &PermissionCatalogRepository{
		pool: pool,
	}
}

func (p *PermissionCatalogRepository) List(ctx context.Context) ([]application.PermissionCatalogEntry, error) {
	rows, err := p.pool.Query(
		ctx,
		`
			SELECT code, resource, action, description
			FROM permissions
			ORDER BY code
		`,
	)

	if err != nil {
		return nil, fmt.Errorf("query permission catalog: %w", err)
	}
	defer rows.Close()

	permissions := make([]application.PermissionCatalogEntry, 0)

	for rows.Next() {
		var permission application.PermissionCatalogEntry

		if err := rows.Scan(
			&permission.Code,
			&permission.Resource,
			&permission.Action,
			&permission.Description,
		); err != nil {
			return nil, fmt.Errorf("scan permission catalog row: %w", err)
		}

		permissions = append(permissions, permission)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate permission catalog : %w", err)
	}

	return permissions, nil
}
