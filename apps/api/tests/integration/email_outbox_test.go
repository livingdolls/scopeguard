//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	encryptionadapter "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/adapters/outbound/encryption"
	identitypostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/adapters/outbound/postgres"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
)

func newVerificationOutboxForTest(
	t *testing.T,
	pool *pgxpool.Pool,
) (
	*identitypostgres.EmailVerificationOutboxRepository,
	*encryptionadapter.AESGCM,
) {
	t.Helper()

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}

	sealer, err := encryptionadapter.NewAESGCM(
		key,
		"test-v1",
	)
	if err != nil {
		t.Fatal(err)
	}

	return identitypostgres.NewEmailVerificationOutboxRepository(
		pool,
		sealer,
	), sealer
}

func readOutboxToken(
	t *testing.T,
	pool *pgxpool.Pool,
	sealer *encryptionadapter.AESGCM,
	eventID uuid.UUID,
) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	var encrypted []byte
	var keyID string

	err := pool.QueryRow(
		ctx,
		`SELECT encrypted_payload, encrypted_key_id
         FROM email_outbox
         WHERE id = $1`,
		eventID,
	).Scan(&encrypted, &keyID)

	if err != nil {
		t.Fatal(err)
	}

	message, err := sealer.Open(
		eventID,
		application.EncryptedEmailPayload{
			KeyID: keyID,
			Data:  encrypted,
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	return message.Token
}

func TestEmailOutboxAtomicIssuance(t *testing.T) {
	pool, userRepo, userID := setupVerificationUser(t)

	outboxRepo, sealer := newVerificationOutboxForTest(
		t, pool,
	)

	issuer := application.NewIssueEmailVerificationService(
		outboxRepo,
	)
	verifier := application.NewVerifyEmailService(userRepo)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer cancel()

	eventID, err := issuer.Execute(ctx, userID)
	if err != nil {
		t.Fatalf("issue verification: %v", err)
	}

	var tokenCount int
	var outboxCount int

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
         FROM email_verification_tokens
         WHERE user_id = $1`,
		userID,
	).Scan(&tokenCount)
	if err != nil {
		t.Fatal(err)
	}

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
         FROM email_outbox o
         JOIN email_verification_tokens v
           ON v.id = o.verification_token_id
         WHERE o.id = $1
           AND v.user_id = $2
           AND o.status = 'pending'`,
		eventID,
		userID,
	).Scan(&outboxCount)
	if err != nil {
		t.Fatal(err)
	}

	if tokenCount != 1 || outboxCount != 1 {
		t.Fatalf(
			"expected token and outbox, got %d/%d",
			tokenCount, outboxCount,
		)
	}

	// Decrypt in the test only, simulating future worker.
	rawToken := readOutboxToken(
		t, pool, sealer, eventID,
	)

	if err := verifier.Execute(ctx, rawToken); err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	allowed, err := userRepo.CanAuthenticate(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("verified user must be active")
	}
}

type invalidOutboxSealer struct{}

func (invalidOutboxSealer) Seal(
	eventID uuid.UUID,
	email string,
	token string,
) (application.EncryptedEmailPayload, error) {
	return application.EncryptedEmailPayload{
		KeyID: "",
		Data:  make([]byte, 40),
	}, nil
}

func TestEmailOutboxRollback(t *testing.T) {
	pool, _, userID := setupVerificationUser(t)

	repo := identitypostgres.NewEmailVerificationOutboxRepository(
		pool,
		invalidOutboxSealer{},
	)

	issuer := application.NewIssueEmailVerificationService(repo)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer cancel()

	eventID, err := issuer.Execute(ctx, userID)
	if err == nil {
		t.Fatal("expected outbox insertion failure")
	}

	if !strings.Contains(err.Error(), "email_outbox_key_not_blank") {
		t.Fatalf("expected outbox key constraint failure, got %v", err)
	}

	if eventID != uuid.Nil {
		t.Fatal("failed issuance must not return an event ID")
	}

	// The token insert must also have been rolled back.
	var count int

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
         FROM email_verification_tokens
         WHERE user_id = $1`,
		userID,
	).Scan(&count)

	if err != nil {
		t.Fatal(err)
	}

	if count != 0 {
		t.Fatalf(
			"orphan verification token persisted: %d",
			count,
		)
	}
}
