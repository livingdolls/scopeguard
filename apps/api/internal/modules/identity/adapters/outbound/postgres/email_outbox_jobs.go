package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
)

var ErrEmailOutboxLeaseLost = errors.New("email outbox lease no longer owned")

type EmailOutboxJobRepository struct {
	pool *pgxpool.Pool
}

var _ application.EmailOutboxJobRepository = (*EmailOutboxJobRepository)(nil)

func NewEmailOutboxJobRepository(pool *pgxpool.Pool) *EmailOutboxJobRepository {
	return &EmailOutboxJobRepository{
		pool: pool,
	}
}

func (e *EmailOutboxJobRepository) Claim(ctx context.Context, lease time.Duration, maxAttempts int) (application.EmailOutboxJob, bool, error) {
	if lease < time.Second || maxAttempts < 1 {
		return application.EmailOutboxJob{}, false, errors.New("invalid claim configuration")
	}

	// jobs abandoned on their final attempt become dead
	_, err := e.pool.Exec(
		ctx,
		`UPDATE email_outbox
		SET status = 'dead',
		    lease_id = NULL,
		    locked_until = NULL,
		    updated_at = clock_timestamp()
		WHERE status = 'processing'
			AND locked_until <= clock_timestamp()
			AND attempts >= $1
		`,
		maxAttempts,
	)

	if err != nil {
		return application.EmailOutboxJob{}, false, fmt.Errorf("expire exhausted leases: %w", err)
	}

	leaseID := uuid.New()

	const query = `
		WITH candidate AS (
			SELECT id
			FROM email_outbox
			WHERE attempts < $1
			AND (
				(status = 'pending' AND available_at <= clock_timestamp()) 
				OR
				(status = 'processing' AND locked_until <= clock_timestamp())
			)
			ORDER by available_at, created_at
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)

		UPDATE email_outbox AS o
		SET status = 'processing',
			attempts = o.attempts + 1,
			lease_id = $2,
			locked_until = clock_timestamp() + ($3::bigint * INTERVAL '1 second'),
			updated_at = clock_timestamp()
		FROM candidate
		WHERE o.id = candidate.id
		RETURNING
			o.id,
			o.attempts,
			o.encrypted_payload,
			o.encrypted_key_id
	`

	job := application.EmailOutboxJob{
		LeaseID: leaseID,
	}

	err = e.pool.QueryRow(
		ctx,
		query,
		maxAttempts,
		leaseID,
		int64(lease/time.Second),
	).Scan(
		&job.ID,
		&job.Attempts,
		&job.Payload.Data,
		&job.Payload.KeyID,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return application.EmailOutboxJob{}, false, nil
	}

	if err != nil {
		return application.EmailOutboxJob{}, false, fmt.Errorf("claim email job: %w", err)
	}

	return job, true, nil
}

func (e *EmailOutboxJobRepository) CanSend(ctx context.Context, job application.EmailOutboxJob) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1
			FROM email_outbox o 
			JOIN email_verification_tokens v
				ON v.id = o.verification_token_id
			JOIN users u 
				ON u.id = v.user_id
			WHERE o.id = $1
				AND o.lease_id = $2
				AND o.status = 'processing'
				AND o.locked_until > clock_timestamp()
				AND v.consumed_at IS NULL
				AND v.expires_at > clock_timestamp()
				AND u.status = 'pending_verification'
				AND u.email_verified_at IS NULL
		)
	`

	var allowed bool

	err := e.pool.QueryRow(
		ctx,
		query,
		job.ID,
		job.LeaseID,
	).Scan(&allowed)

	if err != nil {
		return false, fmt.Errorf("check verification delivery eligibility: %w", err)
	}

	return allowed, nil
}

func (e *EmailOutboxJobRepository) MarkSent(ctx context.Context, job application.EmailOutboxJob) error {
	return e.updateOwnedJob(
		ctx,
		job,
		`UPDATE email_outbox
		SET status = 'sent',
		    sent_at = clock_timestamp(),
		    lease_id = NULL,
		    locked_until = NULL,
		    updated_at = clock_timestamp()
		WHERE  id = $1
			AND lease_id = $2
			AND status = 'processing'
		`,
	)
}

func (e *EmailOutboxJobRepository) MarkDead(ctx context.Context, job application.EmailOutboxJob) error {
	return e.updateOwnedJob(
		ctx,
		job,
		`UPDATE email_outbox
		SET status = 'dead',
		    lease_id = NULL,
		    locked_until = NULL,
		    updated_at = clock_timestamp()
		WHERE  id = $1
			AND lease_id = $2
			AND status = 'processing'
		`,
	)
}

func (e *EmailOutboxJobRepository) MarkRetry(ctx context.Context, job application.EmailOutboxJob, availableAt time.Time) error {
	result, err := e.pool.Exec(
		ctx,
		`
		UPDATE email_outbox
		SET status = 'pending',
		    available_at = $3,
		    lease_id = NULL,
		    locked_until = NULL,
		    updated_at = clock_timestamp()
		WHERE  id = $1
			AND lease_id = $2
			AND status = 'processing'
	`,
		job.ID,
		job.LeaseID,
		availableAt,
	)

	return checkJobUpdate(result.RowsAffected(), err)
}

func (e *EmailOutboxJobRepository) updateOwnedJob(ctx context.Context, job application.EmailOutboxJob, query string) error {
	result, err := e.pool.Exec(
		ctx,
		query,
		job.ID,
		job.LeaseID,
	)

	if err != nil {
		return fmt.Errorf("update email outbox job: %w", err)
	}

	return checkJobUpdate(result.RowsAffected(), nil)
}

func checkJobUpdate(rows int64, err error) error {
	if err != nil {
		return fmt.Errorf("update email outbox job: %w", err)
	}

	if rows != 1 {
		return ErrEmailOutboxLeaseLost
	}

	return nil
}
