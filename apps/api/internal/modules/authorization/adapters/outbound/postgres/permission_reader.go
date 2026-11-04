package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
)

type PermissionReader struct {
	pool *pgxpool.Pool
}

var _ application.PermissionReader = (*PermissionReader)(nil)

func NewPermissionReader(pool *pgxpool.Pool) *PermissionReader {
	return &PermissionReader{pool: pool}
}

func (p *PermissionReader) HasPermission(ctx context.Context, userID uuid.UUID, organizationID uuid.UUID, permissionCode string) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 
			FROM users u
			JOIN organization_memberships m
				ON m.user_id = u.id
			JOIN membership_roles mr
				ON mr.organization_id = m.organization_id
				AND mr.membership_id = m.id
			JOIN role_permissions rp
				ON rp.organization_id = mr.organization_id
				AND rp.role_id = mr.role_id
			WHERE u.id = $1
				AND m.organization_id = $2
				AND rp.permission_code = $3
			AND u.status = 'active'
			AND u.email_verified_at IS NOT NULL
		)
	`

	var allowed bool

	err := p.pool.QueryRow(ctx, query, userID, organizationID, permissionCode).Scan(&allowed)
	if err != nil {
		return false, fmt.Errorf("query effective permission: %w", err)
	}

	return allowed, nil
}
