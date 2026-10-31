package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
)

type RolePermissionRepository struct {
	pool *pgxpool.Pool
}

func NewRolePermissionRepository(pool *pgxpool.Pool) *RolePermissionRepository {
	return &RolePermissionRepository{
		pool: pool,
	}
}

func (r *RolePermissionRepository) Grant(ctx context.Context, actorUserID uuid.UUID, organizationID uuid.UUID, roleID uuid.UUID, permissionCode string) (bool, error) {
	tx, rollback, err := beginAuthorizationTx(ctx, r.pool)
	if err != nil {
		return false, err
	}

	defer rollback()

	if err := authorizeRolePermissionMutation(
		ctx,
		tx,
		actorUserID,
		organizationID,
		roleID,
		permissionCode,
	); err != nil {
		return false, err
	}

	result, err := tx.Exec(
		ctx,
		`INSERT INTO role_permissions (
			organization_id,
            role_id,
            permission_code
		)
		VALUES ($1, $2, $3)
		ON CONFLICT (
		    organization_id,
		    role_id,
		    permission_code
		)
		DO NOTHING 
		`,
		organizationID,
		roleID,
		permissionCode,
	)

	if err != nil {
		return false, fmt.Errorf("insert role permission: %w", err)
	}

	added := result.RowsAffected() == 1

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit permission assignment: %w", err)
	}

	return added, nil
}

func (r *RolePermissionRepository) Revoke(ctx context.Context, actorUserID uuid.UUID, organizationID uuid.UUID, roleID uuid.UUID, permissionCode string) (bool, error) {
	tx, rollback, err := beginAuthorizationTx(ctx, r.pool)
	if err != nil {
		return false, err
	}

	defer rollback()

	if err := authorizeRolePermissionMutation(
		ctx,
		tx,
		actorUserID,
		organizationID,
		roleID,
		permissionCode,
	); err != nil {
		return false, err
	}

	result, err := tx.Exec(
		ctx,
		`
			DELETE FROM role_permissions
			WHERE organization_id = $1
			AND role_id = $2
			AND permission_code = $3
		`,
		organizationID,
		roleID,
		permissionCode,
	)

	if err != nil {
		return false, fmt.Errorf("delete role permission: %w", err)
	}

	removed := result.RowsAffected() == 1

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit role permission revoke: %w", err)
	}

	return removed, nil
}

var _ application.RolePermissionRepository = (*RolePermissionRepository)(nil)
var _ application.RolePermissionRevoker = (*RolePermissionRepository)(nil)
