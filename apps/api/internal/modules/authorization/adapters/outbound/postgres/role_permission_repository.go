package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
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
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin permission transaction: %w", err)
	}

	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := tx.Rollback(rollbackCtx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			fmt.Printf("rollback permission transaction: %v\n", err)
		}
	}()

	// lock organization before all authorization check
	var lockedOrganizationID uuid.UUID

	err = tx.QueryRow(
		ctx,
		`SELECT id FROM organizations WHERE id = $1`,
		organizationID,
	).Scan(&lockedOrganizationID)

	if errors.Is(err, pgx.ErrNoRows) {
		return false, application.ErrRoleOrganizationNotFound
	}

	if err != nil {
		return false, fmt.Errorf("lock organization: %w", err)
	}

	var membershipID uuid.UUID

	err = tx.QueryRow(
		ctx,
		`
		SELECT m.id
		FROM organization_memberships m 
		JOIN users u 
			ON u.id = m.user_id
		WHERE m.organization_id = $1
		AND m.user_id = $2
		AND u.status = 'active'
		AND u.email_verified_at IS NOT NULL
		FOR SHARE of m, u
		`,
		organizationID,
		actorUserID,
	).Scan(&membershipID)

	if errors.Is(err, pgx.ErrNoRows) {
		return false, application.ErrPermissionAssignmentForbidden
	}

	if err != nil {
		return false, fmt.Errorf("check actor membership: %w", err)
	}

	const actorPermissionQuery = `
		SELECT
			COALESCE(
				BOOL_OR(rp.permission_code = $3),
				FALSE
			) AS can_manage_roles,
		    
		    COALESCE(
		    	BOOL_OR(rp.permission_code = $4),
			    FALSE
		    ) AS can_delegate_permissions
		FROM membership_roles mr
		JOIN role_permissions rp 
			ON rp.organization_id = mr.organization_id
		AND rp.role_id = mr.role_id
		WHERE mr.organization_id = $1
			AND mr.membership_id = $2
	`

	var canManageRoles bool
	var canDelegate bool

	err = tx.QueryRow(
		ctx,
		actorPermissionQuery,
		organizationID,
		membershipID,
		domain.PermissionRoleUpdate,
		permissionCode,
	).Scan(&canManageRoles, &canDelegate)

	if err != nil {
		return false, fmt.Errorf("check actor permission: %w", err)
	}

	if !canManageRoles {
		return false, application.ErrPermissionAssignmentForbidden
	}

	//verify that the target role is actually in this tenant.
	var isSystem bool

	err = tx.QueryRow(
		ctx,
		`
			SELECT is_system
			FROM roles
			WHERE organization_id = $1
				AND id = $2
			FOR UPDATE
		`,
		organizationID,
		roleID,
	).Scan(&isSystem)

	if errors.Is(err, pgx.ErrNoRows) {
		return false, application.ErrTargetRoleNotFound
	}

	if err != nil {
		return false, fmt.Errorf("load target role: %w", err)
	}

	if isSystem {
		return false, application.ErrSystemRoleImmutable
	}

	var permissionExists bool

	err = tx.QueryRow(
		ctx,
		`
			SELECT EXISTS (
			SELECT 1
			FROM permissions
			WHERE code = $1
		)
		`,

		permissionCode,
	).Scan(&permissionExists)

	if err != nil {
		return false, fmt.Errorf("check permission catalog: %w", err)
	}

	if !permissionExists {
		return false, application.ErrTargetRoleNotFound
	}

	//actor cannot delegate a permission they do not have
	if !canDelegate {
		return false, application.ErrPermissionAssignmentForbidden
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

var _ application.RolePermissionRepository = (*RolePermissionRepository)(nil)
