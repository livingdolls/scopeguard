package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
)

type RoleReader struct {
	pool *pgxpool.Pool
}

var _ application.RoleReader = (*RoleReader)(nil)

func NewRoleReader(pool *pgxpool.Pool) *RoleReader {
	return &RoleReader{
		pool: pool,
	}
}

func (r *RoleReader) ListByOrganization(ctx context.Context, organizationID uuid.UUID) ([]application.RoleListItem, error) {
	const query = `
		SELECT
		    id,
		    organization_id,
		    name,
		    slug,
		    is_system,
		    created_at,
		    updated_at
		FROM roles
		WHERE organization_id = $1
		ORDER BY is_system DESC, slug ASC 
	`

	rows, err := r.pool.Query(ctx, query, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list organization roles: %w", err)
	}

	defer rows.Close()

	roles := make([]application.RoleListItem, 0)

	for rows.Next() {
		var role application.RoleListItem

		if err := rows.Scan(
			&role.ID,
			&role.OrganizationID,
			&role.Name,
			&role.Slug,
			&role.IsSystem,
			&role.CreatedAt,
			&role.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan organization role: %w", err)
		}

		roles = append(roles, role)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate organization roles: %w", err)
	}

	return roles, nil
}
