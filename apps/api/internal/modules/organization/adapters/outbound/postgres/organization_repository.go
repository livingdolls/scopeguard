package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/organization/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/organization/domain"
)

const (
	uniqueSlugConstraint = "organizations_slug_key"
	membershipUserFK     = "organization_memberships_user_fk"
)

type OrganizationRepository struct {
	pool *pgxpool.Pool
}

var _ application.OrganizationRepository = (*OrganizationRepository)(nil)

func NewOrganizationRepository(pool *pgxpool.Pool) *OrganizationRepository {
	return &OrganizationRepository{
		pool: pool,
	}
}

func (o OrganizationRepository) CreateWithOwner(ctx context.Context, org *domain.Organization, ownerUserID uuid.UUID) error {
	// begin tx
	tx, err := o.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin organization transaction: %w", err)
	}

	// rollback if the transaction was not committed
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)

		defer cancel()

		_ = tx.Rollback(rollbackCtx)
	}()

	// insert organization
	const query = `
		INSERT INTO organizations (
		    id,
		    name,
		    slug,
		    created_at,
		    updated_at
			) VALUES ($1, $2, $3, $4, $5)
		`

	_, err = tx.Exec(
		ctx,
		query,
		org.ID().String(),
		org.Name(),
		org.Slug(),
		org.CreatedAt(),
		org.UpdatedAt(),
	)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == uniqueSlugConstraint {
			return fmt.Errorf("insert organization: %w", application.ErrOrganizationSlugAlreadyExists)
		}

		return fmt.Errorf("insert organization: %w", err)
	}

	// insert owner membership on the same tx
	const insertOwnerMembership = `
		INSERT INTO organization_memberships (
		                                       organization_id,
		                                       user_id,
		                                       role,
		                                       created_at,
		                                       updated_at
		                                       ) VALUES ($1, $2, 'owner', $3, $4)
		`

	_, err = tx.Exec(
		ctx,
		insertOwnerMembership,
		org.ID().String(),
		ownerUserID.String(),
		org.CreatedAt(),
		org.UpdatedAt(),
	)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == membershipUserFK {
			return fmt.Errorf("insert owner membership: %w", application.ErrOwnerUserNotFound)
		}

		return fmt.Errorf("insert owner membership: %w", err)
	}

	// commit only after both succeed
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit organization transaction: %w", err)
	}

	return nil
}
