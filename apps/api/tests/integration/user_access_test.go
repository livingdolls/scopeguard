//go:build integration

package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"

	identitypostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/adapters/outbound/postgres"
)

func TestUserAccessChecker(t *testing.T) {
	pool := openTestDB(t)
	repo := identitypostgres.NewUserRepository(pool)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	userID := uuid.New()
	email := "access-" + uuid.NewString() + "@example.test"

	t.Cleanup(func() {
		cleanupCtx, done := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer done()

		_, err := pool.Exec(
			cleanupCtx,
			"DELETE FROM users WHERE id = $1",
			userID,
		)
		if err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	})

	check := func(want bool) {
		t.Helper()

		allowed, err := repo.CanAuthenticate(ctx, userID)
		if err != nil {
			t.Fatal(err)
		}

		if allowed != want {
			t.Fatalf(
				"expected allowed=%t, got %t",
				want, allowed,
			)
		}
	}

	// Missing user.
	check(false)

	// A newly registered user starts pending.
	_, err := pool.Exec(
		ctx,
		"INSERT INTO users (id, email) VALUES ($1, $2)",
		userID,
		email,
	)
	if err != nil {
		t.Fatal(err)
	}

	check(false)

	// Simulate a completed verification in test data.
	_, err = pool.Exec(
		ctx,
		`UPDATE users
         SET status = 'active',
             email_verified_at = NOW()
         WHERE id = $1`,
		userID,
	)
	if err != nil {
		t.Fatal(err)
	}

	check(true)

	// Suspend the account.
	_, err = pool.Exec(
		ctx,
		`UPDATE users
         SET status = 'suspended'
         WHERE id = $1`,
		userID,
	)
	if err != nil {
		t.Fatal(err)
	}

	check(false)
}

func TestFindActiveUserProfile(t *testing.T) {
	pool, repo, userID := setupVerificationUser(t)

	ctx := context.Background()

	// Pending user must not have an accessible profile.
	_, err := repo.FindProfileByID(ctx, userID)
	if !errors.Is(err, application.ErrUserNotFound) {
		t.Fatalf("expected inaccessible pending user, got %v", err)
	}

	_, err = pool.Exec(
		ctx,
		`UPDATE users
         SET status = 'active',
             email_verified_at = NOW()
         WHERE id = $1`,
		userID,
	)
	if err != nil {
		t.Fatal(err)
	}

	profile, err := repo.FindProfileByID(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}

	if profile.ID != userID || profile.Email == "" {
		t.Fatal("unexpected user profile")
	}
}
