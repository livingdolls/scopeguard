package postgres

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/domain"
)

type EmailVerificationOutboxRepository struct {
	pool   *pgxpool.Pool
	sealer application.EmailPayloadSealer
}

var _ application.EmailVerificationOutbox = (*EmailVerificationOutboxRepository)(nil)

func NewEmailVerificationOutboxRepository(pool *pgxpool.Pool, sealer application.EmailPayloadSealer) *EmailVerificationOutboxRepository {
	return &EmailVerificationOutboxRepository{
		pool:   pool,
		sealer: sealer,
	}
}

func (e EmailVerificationOutboxRepository) Enqueue(ctx context.Context, eventID uuid.UUID, userID uuid.UUID, rawToken string, tokenHash []byte, expiresAt time.Time) error {
	if eventID == uuid.Nil || userID == uuid.Nil || rawToken == "" || len(tokenHash) != sha256.Size || !expiresAt.After(time.Now()) {
		return errors.New("invalid  verification outbox input")
	}

	tx, err := e.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_ = tx.Rollback(rollbackCtx)
	}()

	// lock user and obtain the canonical email
	var email, status string
	err = tx.QueryRow(
		ctx,
		`SELECT email, status FROM users WHERE id = $1 FOR UPDATE`, userID,
	).Scan(&email, &status)

	if errors.Is(err, pgx.ErrNoRows) {
		return application.ErrVerificationNotAllowed
	}

	if err != nil {
		return fmt.Errorf("load verification user: %w", err)
	}

	if status != string(domain.AccountPendingVerification) {
		return application.ErrVerificationNotAllowed
	}

	//insert token hash
	verificationTokenID := uuid.New()

	_, err = tx.Exec(
		ctx,
		`INSERT INTO email_verification_tokens (
			id,
			user_id,
			token_hash,
			expires_at
			) VALUES ($1, $2, $3, $4)`,
		verificationTokenID,
		userID,
		tokenHash,
		expiresAt,
	)

	if err != nil {
		return fmt.Errorf("insert verification token: %w", err)
	}

	//encrypt delivery method
	encrypted, err := e.sealer.Seal(eventID, email, rawToken)
	if err != nil {
		return fmt.Errorf("failed to seal email verification token: %w", err)
	}

	// insert the outbox event in the same transaction
	_, err = tx.Exec(
		ctx,
		`INSERT INTO email_outbox (
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
			)`,
		eventID,
		verificationTokenID,
		encrypted.Data,
		encrypted.KeyID,
	)

	if err != nil {
		return fmt.Errorf("insert verification outbox: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
