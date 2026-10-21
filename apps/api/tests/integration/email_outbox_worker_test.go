//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	identitypostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/adapters/outbound/postgres"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
)

func TestEmailOutboxClaimAndLeaseRecovery(t *testing.T) {
	pool, _, userID := setupVerificationUser(t)

	outbox, _ := newVerificationOutboxForTest(t, pool)
	issuer := application.NewIssueEmailVerificationService(outbox)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer cancel()

	eventID, err := issuer.Execute(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}

	repo := identitypostgres.NewEmailOutboxJobRepository(pool)

	first, found, err := repo.Claim(
		ctx, 2*time.Minute, 5,
	)
	if err != nil || !found {
		t.Fatalf("first claim failed: %v", err)
	}

	if first.ID != eventID || first.Attempts != 1 {
		t.Fatal("unexpected first claim")
	}

	// A second worker cannot claim a live lease.
	_, found, err = repo.Claim(
		ctx, 2*time.Minute, 5,
	)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("job with active lease was reclaimed")
	}

	// Simulate a crashed worker by expiring the lease.
	_, err = pool.Exec(
		ctx,
		`UPDATE email_outbox
         SET locked_until = NOW() - INTERVAL '1 second'
         WHERE id = $1`,
		eventID,
	)
	if err != nil {
		t.Fatal(err)
	}

	second, found, err := repo.Claim(
		ctx, 2*time.Minute, 5,
	)
	if err != nil || !found {
		t.Fatalf("reclaim failed: %v", err)
	}

	if second.ID != first.ID ||
		second.LeaseID == first.LeaseID ||
		second.Attempts != 2 {
		t.Fatal("expected new lease and incremented attempt")
	}

	// The old worker must not finalize the new lease.
	err = repo.MarkSent(ctx, first)

	if !errors.Is(
		err,
		identitypostgres.ErrEmailOutboxLeaseLost,
	) {
		t.Fatalf("expected stale lease rejection, got %v", err)
	}

	// Current lease owner can finish the job.
	if err := repo.MarkSent(ctx, second); err != nil {
		t.Fatal(err)
	}

	var status string

	err = pool.QueryRow(
		ctx,
		"SELECT status FROM email_outbox WHERE id = $1",
		eventID,
	).Scan(&status)
	if err != nil {
		t.Fatal(err)
	}

	if status != "sent" {
		t.Fatalf("expected sent, got %s", status)
	}
}

func TestEmailOutboxCannotSendExpiredToken(t *testing.T) {
	pool, _, userID := setupVerificationUser(t)

	outbox, _ := newVerificationOutboxForTest(t, pool)
	issuer := application.NewIssueEmailVerificationService(outbox)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer cancel()

	eventID, err := issuer.Execute(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}

	repo := identitypostgres.NewEmailOutboxJobRepository(pool)

	job, found, err := repo.Claim(
		ctx, 2*time.Minute, 5,
	)
	if err != nil || !found || job.ID != eventID {
		t.Fatalf("claim failed: %v", err)
	}

	// Simulate expiration without sleeping.
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

	allowed, err := repo.CanSend(ctx, job)
	if err != nil {
		t.Fatal(err)
	}

	if allowed {
		t.Fatal("expired verification must not be sent")
	}

	if err := repo.MarkDead(ctx, job); err != nil {
		t.Fatal(err)
	}
}
