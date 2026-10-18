//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	identitypostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/adapters/outbound/postgres"
)

func TestUserExistenceChecker(t *testing.T) {
	pool := openTestDB(t)
	repository := identitypostgres.NewUserRepository(pool)

	userID := uuid.New()
	email := "auth-check-" + uuid.NewString() + "@example.test"

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

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

	// Before insertion, user must not exist.
	exists, err := repository.Exists(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}

	if exists {
		t.Fatal("unexpected existing user")
	}

	_, err = pool.Exec(
		ctx,
		"INSERT INTO users (id, email) VALUES ($1, $2)",
		userID,
		email,
	)
	if err != nil {
		t.Fatalf("insert test user: %v", err)
	}

	// After insertion, user must exist.
	exists, err = repository.Exists(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}

	if !exists {
		t.Fatal("expected user to exist")
	}
}
