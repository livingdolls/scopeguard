//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	identitypostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/adapters/outbound/postgres"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
)

func setupVerificationUser(
	t *testing.T,
) (*pgxpool.Pool, *identitypostgres.UserRepository, uuid.UUID) {
	t.Helper()

	pool := openTestDB(t)
	repo := identitypostgres.NewUserRepository(pool)

	userID := uuid.New()
	email := "verify-" + uuid.NewString() + "@example.test"

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	_, err := pool.Exec(
		ctx,
		`INSERT INTO users (
            id, email, status
         )
         VALUES ($1, $2, 'pending_verification')`,
		userID,
		email,
	)
	if err != nil {
		t.Fatalf("create pending test user: %v", err)
	}

	cleanupRegisteredUser(t, pool, userID)

	return pool, repo, userID
}

func TestEmailVerificationLifecycle(t *testing.T) {
	pool, repo, userID := setupVerificationUser(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer cancel()

	outboxRepo, sealer := newVerificationOutboxForTest(
		t, pool,
	)
	issuer := application.NewIssueEmailVerificationService(outboxRepo)
	verifier := application.NewVerifyEmailService(repo)

	firstEventID, err := issuer.Execute(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}

	firstToken := readOutboxToken(
		t, pool, sealer, firstEventID,
	)

	secondEventID, err := issuer.Execute(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}

	secondToken := readOutboxToken(
		t, pool, sealer, secondEventID,
	)

	if firstToken == secondToken {
		t.Fatal("tokens must be unique")
	}

	// Activate the account using the first token.
	if err := verifier.Execute(ctx, firstToken); err != nil {
		t.Fatalf("verify email: %v", err)
	}

	var status string
	var verifiedAt *time.Time

	err = pool.QueryRow(
		ctx,
		`SELECT status, email_verified_at
         FROM users WHERE id = $1`,
		userID,
	).Scan(&status, &verifiedAt)

	if err != nil {
		t.Fatal(err)
	}

	if status != "active" || verifiedAt == nil {
		t.Fatal("verification did not activate account")
	}

	// Both tokens must now be invalid.
	for _, token := range []string{firstToken, secondToken} {
		err := verifier.Execute(ctx, token)

		if !errors.Is(
			err,
			application.ErrInvalidVerificationToken,
		) {
			t.Fatalf("expected consumed token rejection: %v", err)
		}
	}

	var total, consumed int

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*), COUNT(consumed_at)
         FROM email_verification_tokens
         WHERE user_id = $1`,
		userID,
	).Scan(&total, &consumed)

	if err != nil {
		t.Fatal(err)
	}

	if total != 2 || consumed != 2 {
		t.Fatalf(
			"expected 2 consumed tokens, got %d/%d",
			consumed, total,
		)
	}

	// An active user must not receive another token.
	_, err = issuer.Execute(ctx, userID)

	if !errors.Is(
		err,
		application.ErrVerificationNotAllowed,
	) {
		t.Fatalf("expected issuance rejection, got %v", err)
	}
}

func TestEmailVerificationExpired(t *testing.T) {
	pool, repo, userID := setupVerificationUser(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer cancel()

	outboxRepo, sealer := newVerificationOutboxForTest(
		t, pool,
	)
	issuer := application.NewIssueEmailVerificationService(outboxRepo)
	verifier := application.NewVerifyEmailService(repo)

	eventID, err := issuer.Execute(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}

	token := readOutboxToken(
		t, pool, sealer, eventID,
	)

	// Simulate an expired token without sleeping.
	_, err = pool.Exec(
		ctx,
		`UPDATE email_verification_tokens
         SET created_at = NOW() - INTERVAL '48 hours',
             expires_at = NOW() - INTERVAL '24 hours'
         WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		t.Fatal(err)
	}

	err = verifier.Execute(ctx, token)

	if !errors.Is(
		err,
		application.ErrInvalidVerificationToken,
	) {
		t.Fatalf("expected expired token rejection: %v", err)
	}

	var status string

	err = pool.QueryRow(
		ctx,
		"SELECT status FROM users WHERE id = $1",
		userID,
	).Scan(&status)

	if err != nil {
		t.Fatal(err)
	}

	if status != "pending_verification" {
		t.Fatal("expired token must not activate user")
	}
}

func TestEmailVerificationConcurrentUse(t *testing.T) {
	pool, repo, userID := setupVerificationUser(t)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer cancel()

	outboxRepo, sealer := newVerificationOutboxForTest(
		t, pool,
	)
	issuer := application.NewIssueEmailVerificationService(outboxRepo)
	verifier := application.NewVerifyEmailService(repo)

	eventID, err := issuer.Execute(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}

	token := readOutboxToken(
		t, pool, sealer, eventID,
	)

	start := make(chan struct{})
	results := make(chan error, 2)

	for i := 0; i < 2; i++ {
		go func() {
			<-start
			results <- verifier.Execute(ctx, token)
		}()
	}

	close(start)

	successes := 0
	rejected := 0

	for i := 0; i < 2; i++ {
		err := <-results

		switch {
		case err == nil:
			successes++

		case errors.Is(
			err,
			application.ErrInvalidVerificationToken,
		):
			rejected++

		default:
			t.Fatalf("unexpected verification error: %v", err)
		}
	}

	if successes != 1 || rejected != 1 {
		t.Fatalf(
			"expected one success and one rejection, got %d/%d",
			successes, rejected,
		)
	}
}
