package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/domain"
)

const uniqueEmailIndex = "users_email_lower_unique"

type UserRepository struct {
	pool *pgxpool.Pool
}

var _ application.RegistrationRepository = (*UserRepository)(nil)

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{
		pool: pool,
	}
}

func (u UserRepository) CreateWithPassword(ctx context.Context, user *domain.User, passwordHash string) error {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("start transaction: %w", err)
	}

	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = tx.Rollback(rollbackCtx)
	}()

	const insertUsr = `
		INSERT INTO users (id, email, created_at, updated_at)
		VALUES ($1, $2, $3, $4)
	`

	_, err = tx.Exec(
		ctx,
		insertUsr,
		user.ID(),
		user.Email(),
		user.CreatedAt(),
		user.UpdatedAt(),
	)

	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == uniqueEmailIndex {
			return fmt.Errorf("user already exists: %w", application.ErrEmailAlreadyExists)
		}

		return fmt.Errorf("insert user: %w", err)
	}

	const insertCredential = `
		INSERT INTO user_password_credentials (user_id, password_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4)
	`

	_, err = tx.Exec(
		ctx,
		insertCredential,
		user.ID(),
		passwordHash,
		user.CreatedAt(),
		user.UpdatedAt(),
	)

	if err != nil {
		return fmt.Errorf("insert user password_credentials: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
