package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/organization/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/organization/domain"
)

const uniqueSlugConstraint = "organizations_slug_key"

type OrganizationRepository struct {
	pool *pgxpool.Pool
}

var _ application.OrganizationRepository = (*OrganizationRepository)(nil)

func NewOrganizationRepository(pool *pgxpool.Pool) *OrganizationRepository {
	return &OrganizationRepository{
		pool: pool,
	}
}

func (o OrganizationRepository) Create(ctx context.Context, org *domain.Organization) error {
	const query = `
		INSERT INTO organizations (
		    id,
		    name,
		    slug,
		    created_at,
		    updated_at
			) VALUES ($1, $2, $3, $4, $5)
		`

	_, err := o.pool.Exec(
		ctx,
		query,
		org.ID().String(),
		org.Name(),
		org.Slug(),
		org.CreatedAt(),
		org.UpdatedAt(),
	)

	if err == nil {
		return nil
	}

	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		if pgErr.Code == "23505" && pgErr.ConstraintName == uniqueSlugConstraint {
			return fmt.Errorf("create organization: %w", application.ErrOrganizationSlugAlreadyExists)
		}

	}

	return fmt.Errorf("insert organization: %w", err)
}
