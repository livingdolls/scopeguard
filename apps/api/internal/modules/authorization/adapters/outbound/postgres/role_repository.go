package postgres

import (
	"context"
	"errors"
	"fmt"

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

func (r *RoleRepository) Insert(ctx context.Context, role *domain.Role) error {
	if role == nil {
		return errors.New("role entity is required")
	}

	const query = `
		INSERT INTO roles(
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

	_, err := r.pool.Exec(
		ctx,
		query,
		role.ID(),
		role.OrganizationID(),
		role.Name(),
		role.Slug(),
		role.IsSystem(),
		role.CreatedAt(),
		role.UpdatedAt(),
	)

	if err == nil {
		return nil
	}

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
