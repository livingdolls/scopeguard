package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
)

type RolePermissionReader struct {
	pool *pgxpool.Pool
}

var _ application.RolePermissionReader = (*RolePermissionReader)(nil)

func NewRolePermissionReader(pool *pgxpool.Pool) *RolePermissionReader {
	return &RolePermissionReader{
		pool: pool,
	}
}

func (r *RolePermissionReader) ListByRole(ctx context.Context, organizationID uuid.UUID, roleID uuid.UUID) ([]string, error) {
	const query = `
		SELECT COALESCE(
			ARRAY_AGG(
				rp.permission_code
				ORDER BY rp.permission_code
			) FILTER (
                WHERE rp.permission_code IS NOT NULL
			),
		    ARRAY[]::TEXT[]
		)
		FROM roles r
		LEFT JOIN role_permissions rp
			ON rp.organization_id = r.organization_id
			AND rp.role_id = r.id
		WHERE r.organization_id = $1
			AND r.id = $2
		GROUP BY r.id
	`

	permissions := make([]string, 0)

	err := r.pool.QueryRow(ctx, query, organizationID, roleID).Scan(&permissions)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, application.ErrTargetRoleNotFound
	}

	if err != nil {
		return nil, fmt.Errorf("query role permissions: %w", err)
	}

	return permissions, nil
}
