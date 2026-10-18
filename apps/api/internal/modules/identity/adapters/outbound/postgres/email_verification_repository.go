package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/domain"
)

var _ application.EmailVerificationRepository = (*UserRepository)(nil)

func (u UserRepository) StorePending(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) error {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin verification transaction: %w", err)
	}

	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = tx.Rollback(rollbackCtx)
	}()

	// lock the user row to serialize issuance and activation
	var status string

	err = tx.QueryRow(
		ctx,
		`SELECT status
		FROM users
		WHERE id = $1
		FOR UPDATE
	`, userID).Scan(&status)

	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrVerificationNotAllowed
	}

	if err != nil {
		return fmt.Errorf("load verification status: %w", err)
	}

	if status != string(domain.AccountPendingVerification) {
		return application.ErrVerificationNotAllowed
	}

	_, err = tx.Exec(
		ctx,
		`INSERT INTO email_verification_tokens (
            	user_id, token_hash, expires_at
		) VALUES ($1, $2, $3)`,
		userID,
		tokenHash,
		expiresAt,
	)

	if err != nil {
		return fmt.Errorf("insert email_verification_tokens: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit verification transaction: %w", err)
	}

	return nil
}

func (u UserRepository) ConsumeAndActivate(ctx context.Context, tokenHash []byte) error {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin verification transaction: %w", err)
	}

	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = tx.Rollback(rollbackCtx)
	}()

	// identity the user without locking token rows
	var userID uuid.UUID

	err = tx.QueryRow(
		ctx,
		`
		SELECT user_id
		FROM email_verification_tokens
		WHERE token_hash = $1
			AND consumed_at IS NULL
			AND expires_at > clock_timestamp()
		`,
		tokenHash,
	).Scan(&userID)

	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrInvalidVerificationToken
	}

	if err != nil {
		return fmt.Errorf("find verification token: %w", err)
	}

	// atomically activate a still-pending user
	// recheck token validity at the point of activation
	result, err := tx.Exec(
		ctx,
		`UPDATE users
		SET status = 'active',
		    email_verified_at = clock_timestamp(),
		    updated_at = clock_timestamp()
		WHERE id = $1
			AND status = 'pending_verification'
			AND EXISTS (
			    SELECT 1
			    FROM email_verification_tokens
			    WHERE token_hash = $2
			    AND consumed_at IS NULL
			    AND expires_at > clock_timestamp()
			)
		`,
		userID,
		tokenHash,
	)

	if err != nil {
		return fmt.Errorf("activate verified user: %w", err)
	}

	if result.RowsAffected() != 1 {
		return application.ErrInvalidVerificationToken
	}

	// invalidate all outstanding tokens for this user
	_, err = tx.Exec(
		ctx,
		`UPDATE email_verification_tokens
		SET consumed_at = clock_timestamp()
		WHERE user_id = $1
		AND consumed_at IS NULL
		`,
		userID,
	)

	if err != nil {
		return fmt.Errorf("consume verification token: %w", err)
	}

	// commit both changes together
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit verification transaction: %w", err)
	}

	return nil
}
