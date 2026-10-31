package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
)

type MembershipRoleRepository struct {
	pool *pgxpool.Pool
}

var _ application.MembershipRoleRepository = (*MembershipRoleRepository)(nil)

func NewMembershipRoleRepository(pool *pgxpool.Pool) *MembershipRoleRepository {
	return &MembershipRoleRepository{pool: pool}
}

func (m *MembershipRoleRepository) Assign(ctx context.Context, actorUserID uuid.UUID, organizationID uuid.UUID, membershipID uuid.UUID, roleID uuid.UUID) (assigned bool, err error) {
	tx, rollback, err := beginAuthorizationTx(ctx, m.pool)
	if err != nil {
		return false, err
	}
	defer rollback()

	// every RBAC mutations takes the tenant lock first
	var tentantID uuid.UUID

	err = tx.QueryRow(
		ctx,
		`
		SELECT id
		FROM organizations
		WHERE id = $1
		FOR UPDATE
		`,
		organizationID,
	).Scan(&tentantID)

	if errors.Is(err, pgx.ErrNoRows) {
		return false, application.ErrRoleOrganizationNotFound
	}

	if err != nil {
		return false, fmt.Errorf("lock organization: %w", err)
	}

	// make sure actor active and organization member
	var actorMembershipID uuid.UUID

	err = tx.QueryRow(
		ctx,
		`
		SELECT m.id
		FROM organization_memberships m 
		JOIN users u ON u.id = m.user_id
		WHERE m.organization_id = $1
			AND m.user_id = $2
			AND u.status = 'active'
			AND u.email_verified_at IS NOT NULL
		FOR SHARE OF m, u
		`,
		organizationID,
		actorUserID,
	).Scan(&actorMembershipID)

	if errors.Is(err, pgx.ErrNoRows) {
		return false, application.ErrPermissionAssignmentForbidden
	}

	if err != nil {
		return false, fmt.Errorf("check actor membership: %w", err)
	}

	// make sure actor have permission member:role_assign
	var canAssign bool

	err = tx.QueryRow(
		ctx,
		`
		SELECT EXISTS (
			SELECT 1
			FROM membership_roles mr
			JOIN role_permissions rp 
				ON rp.organization_id = mr.organization_id
				AND rp.role_id = mr.role_id
			WHERE mr.organization_id = $1
				AND mr.membership_id = $2
				AND rp.permission_code = $3
		)
		`,
		organizationID,
		actorMembershipID,
		domain.PermissionMemberRoleAssign,
	).Scan(&canAssign)

	if err != nil {
		return false, fmt.Errorf("check role assignment permission: %w", err)
	}

	if !canAssign {
		return false, application.ErrPermissionAssignmentForbidden
	}

	var targetRoleSlug string

	err = tx.QueryRow(
		ctx,
		`
		SELECT slug
		FROM roles
		WHERE organization_id = $1
		AND id = $2
		FOR SHARE 
		`,
		organizationID,
		roleID,
	).Scan(&targetRoleSlug)

	if errors.Is(err, pgx.ErrNoRows) {
		return false, application.ErrTargetRoleNotFound
	}

	if err != nil {
		return false, fmt.Errorf("load target role slug: %w", err)
	}

	if targetRoleSlug == domain.RoleOwner {
		return false, application.ErrOwnerRoleAssignmentForbidden
	}

	// validate target role and membership
	var targetMembershipID uuid.UUID

	err = tx.QueryRow(
		ctx,
		`
		SELECT id
		FROM organization_memberships
		WHERE organization_id = $1
			AND id = $2
		FOR SHARE
		`,
		organizationID,
		membershipID,
	).Scan(&targetMembershipID)

	if errors.Is(err, pgx.ErrNoRows) {
		return false, application.ErrMembershipNotFound
	}

	if err != nil {
		return false, fmt.Errorf("load target membership: %w", err)
	}

	var canDelegateEntireRole bool

	err = tx.QueryRow(
		ctx,
		`
		SELECT NOT EXISTS (
			SELECT 1
			FROM role_permissions target_permission
			WHERE target_permission.organization_id = $1
				AND target_permission.role_id = $2
				AND NOT EXISTS (
				    SELECT 1
				    FROM membership_roles actor_role
				    JOIN role_permissions actor_permission
				    	ON actor_permission.organization_id = actor_role.organization_id
				    AND actor_permission.role_id = actor_role.role_id
				    WHERE actor_role.organization_id = $1
				    	AND actor_role.membership_id = $3
				    	AND actor_permission.permission_code = target_permission.permission_code
				)
		)
		`,
		organizationID,
		roleID,
		actorMembershipID,
	).Scan(&canDelegateEntireRole)

	if err != nil {
		return false, fmt.Errorf("check role permission delegation: %w", err)
	}

	if !canDelegateEntireRole {
		return false, application.ErrPermissionAssignmentForbidden
	}

	result, err := tx.Exec(
		ctx,
		`
		INSERT INTO membership_roles (
		    organization_id,
		    membership_id,
			role_id
		)
		VALUES ($1, $2, $3)
		ON CONFLICT (membership_id, role_id)
		DO NOTHING 
		`,
		organizationID,
		membershipID,
		roleID,
	)

	if err != nil {
		return false, fmt.Errorf("insert membership role: %w", err)
	}

	isAssigned := result.RowsAffected() == 1

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit membership role assignment: %w", err)
	}

	return isAssigned, nil
}
