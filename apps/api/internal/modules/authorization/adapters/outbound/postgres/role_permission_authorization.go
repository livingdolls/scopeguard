package postgres

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
)

func beginAuthorizationTx(ctx context.Context, pool *pgxpool.Pool) (pgx.Tx, func(), error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("begin role permission tx: %w", err)
	}

	rollback := func() {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := tx.Rollback(rollbackCtx); err != nil && err != pgx.ErrTxClosed {
			log.Printf("rollback role permission tx: %v", err)
		}
	}

	return tx, rollback, nil
}

func authorizeRolePermissionMutation(ctx context.Context, tx pgx.Tx, actorUserID uuid.UUID, organizationID uuid.UUID, roleID uuid.UUID, permissionCode string) error {
	// All authorization writers must take this tenant lock before checking rights
	// or mutating assignments. Direct SQL that omits it bypasses serialization;
	// deployments must restrict database privileges and authorization write paths.
	var tenantID uuid.UUID

	err := tx.QueryRow(
		ctx,
		`
			SELECT id
		FROM organizations
			WHERE id = $1
		FOR UPDATE
		`,
		organizationID,
	).Scan(&tenantID)

	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrRoleOrganizationNotFound
	}
	if err != nil {
		return fmt.Errorf("lock organization: %w", err)
	}

	// 2. the actor must be an active, verified member
	var membershipID uuid.UUID

	err = tx.QueryRow(
		ctx,
		`
		SELECT
		    m.id
		FROM organization_memberships m 
		JOIN users u ON u.id = m.user_id
		WHERE m.organization_id = $1
			AND m.user_id = $2
			AND u.status = 'active'
			AND u.email_verified_at IS NOT NULL
		FOR SHARE OF m,u
		`,
		organizationID,
		actorUserID,
	).Scan(&membershipID)

	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrPermissionAssignmentForbidden
	}

	if err != nil {
		return fmt.Errorf("check actor membership: %w", err)
	}

	// combine permissions across the actor's roles
	var canManageRoles bool
	var canDelegate bool

	err = tx.QueryRow(
		ctx,
		`
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
	`,
		organizationID,
		membershipID,
		domain.PermissionRoleUpdate,
		permissionCode,
	).Scan(&canManageRoles, &canDelegate)

	if err != nil {
		return fmt.Errorf("check actor permissions: %w", err)
	}

	if !canManageRoles {
		return application.ErrPermissionAssignmentForbidden
	}

	// target role must belong to this tenant
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
		return application.ErrTargetRoleNotFound
	}

	if err != nil {
		return fmt.Errorf("load target role: %w", err)
	}

	if isSystem {
		return application.ErrSystemRoleImmutable
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
		return fmt.Errorf("check permission catalog: %w", err)
	}

	if !permissionExists {
		return application.ErrPermissionNotFound
	}

	//actor cannot delegate a permission they do not have
	if !canDelegate {
		return application.ErrPermissionAssignmentForbidden
	}

	return nil

}
