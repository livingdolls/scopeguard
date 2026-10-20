package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
)

var _ application.EmailVerificationRepository = (*UserRepository)(nil)

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
