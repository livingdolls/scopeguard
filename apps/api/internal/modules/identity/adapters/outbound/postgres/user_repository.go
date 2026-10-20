package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
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

func (u UserRepository) RegisterWithVerification(ctx context.Context, record application.RegistrationRecord) error {
	if record.User == nil {
		return errors.New("registration user is required")
	}

	if len(record.TokenHash) != sha256.Size {
		return errors.New("invalid verification token hash")
	}

	if record.VerificationID == uuid.Nil || record.OutboxID == uuid.Nil || !record.TokenExpiresAt.After(time.Now()) {
		return errors.New("invalid registration metadata")
	}

	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = tx.Rollback(rollbackCtx)
	}()

	// insert user
	const insertUser = `
		INSERT INTO users (
			id,
			email,
		    status,
		    created_at,
		    updated_at
		)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err = tx.Exec(
		ctx,
		insertUser,
		record.User.ID(),
		record.User.Email(),
		string(record.User.Status()),
		record.User.CreatedAt(),
		record.User.UpdatedAt(),
	)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == uniqueEmailIndex {
			return fmt.Errorf("insert user: %w", application.ErrEmailAlreadyExists)
		}

		return fmt.Errorf("insert user: %w", err)
	}

	// insert password credential
	const insertCredential = `
		INSERT INTO user_password_credentials(
			user_id,
		    password_hash,
		    created_at,
		    updated_at
		)
		VALUES ($1, $2, $3, $4)
	`

	_, err = tx.Exec(
		ctx,
		insertCredential,
		record.User.ID(),
		record.PasswordHash,
		record.User.CreatedAt(),
		record.User.UpdatedAt(),
	)

	if err != nil {
		return fmt.Errorf("insert user password_credentials: %w", err)
	}

	// insert email token hash
	const insertToken = `
		INSERT INTO email_verification_tokens (
			id,
			user_id,
		    token_hash,
		    expires_at
		)
		VALUES ($1, $2, $3, $4)
	`

	_, err = tx.Exec(
		ctx,
		insertToken,
		record.VerificationID,
		record.User.ID(),
		record.TokenHash,
		record.TokenExpiresAt,
	)

	if err != nil {
		return fmt.Errorf("insert email_verification_tokens: %w", err)
	}

	// insert encrypted email outbox
	const insertOutbox = `
		INSERT INTO email_outbox (
			id,
		    verification_token_id,
		    event_type,
		    encrypted_payload,
		    encrypted_key_id
		)
		VALUES (
		        $1, $2,
		        'identity.email_verification',
		        $3, $4
		)
	`

	_, err = tx.Exec(
		ctx,
		insertOutbox,
		record.OutboxID,
		record.VerificationID,
		record.EncryptedPayload.Data,
		record.EncryptedPayload.KeyID,
	)

	if err != nil {
		return fmt.Errorf("insert email_outbox: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
