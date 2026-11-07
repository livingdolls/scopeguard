package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/authorization/domain"
)

const (
	uniqueRoleSlugConstraint = "roles_organization_slug_unique"
	roleOrganizationFK       = "roles_organization_fk"
)

type RoleRepository struct {
	pool *pgxpool.Pool
}

var _ application.RoleRepository = (*RoleRepository)(nil)

func NewRoleRepository(pool *pgxpool.Pool) *RoleRepository {
	return &RoleRepository{
		pool: pool,
	}
}

func (r *RoleRepository) Insert(ctx context.Context, actorUserID uuid.UUID, role *domain.Role) error {
	if role == nil {
		return errors.New("role entity is required")
	}

	if actorUserID == uuid.Nil {
		return application.ErrPermissionAssignmentForbidden
	}

	tx, rollback, err := beginAuthorizationTx(ctx, r.pool)
	if err != nil {
		return err
	}
	defer rollback()

	// serialize RBAC mutations for the organization
	var tenantID uuid.UUID

	err = tx.QueryRow(
		ctx,
		`
		SELECT id
		FROM organizations
		WHERE id = $1
		FOR UPDATE
		`,
		role.OrganizationID(),
	).Scan(&tenantID)

	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrRoleOrganizationNotFound
	}

	if err != nil {
		return fmt.Errorf("lock organization: %w", err)
	}

	// the actor must be active and verified member
	var actorMembershipID uuid.UUID

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
		FOR SHARE OF m,u
		`,
		role.OrganizationID(),
		actorUserID,
	).Scan(&actorMembershipID)

	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrPermissionAssignmentForbidden
	}

	if err != nil {
		return fmt.Errorf("check actor membership: %w", err)
	}

	// actor must have role:create in this tenant
	var canCreateRole bool

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
		role.OrganizationID(),
		actorMembershipID,
		domain.PermissionRoleCreate,
	).Scan(&canCreateRole)

	if err != nil {
		return fmt.Errorf("check role create permissions: %w", err)
	}

	if !canCreateRole {
		return application.ErrPermissionAssignmentForbidden
	}

	// persist custom role
	const insertRole = `
		INSERT INTO roles (
            id,
		    organization_id,
		    name,
			slug,
		    is_system,
		    created_at,
		    updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err = tx.Exec(
		ctx,
		insertRole,
		role.ID(),
		role.OrganizationID(),
		role.Name(),
		role.Slug(),
		role.IsSystem(),
		role.CreatedAt(),
		role.UpdatedAt(),
	)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) {
			switch {
			case pgErr.Code == "23505" && pgErr.ConstraintName == uniqueRoleSlugConstraint:
				return fmt.Errorf("insert role: %w", application.ErrRoleSlugAlreadyExists)

			case pgErr.Code == "23503" && pgErr.ConstraintName == roleOrganizationFK:
				return fmt.Errorf("insert role: %w", application.ErrRoleOrganizationNotFound)
			}
		}

		return fmt.Errorf("insert role: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit role creation: %w", err)
	}

	return nil
}
