//go:build integration

package integration

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/livingdolls/scopeguard/apps/api/internal/platform/ratelimit"
)

func TestRateLimiterSharedAcrossInstances(t *testing.T) {
	pool := openTestDB(t)

	key := bytes.Repeat([]byte{0x42}, 32)

	limiterA, err := ratelimit.NewPostgresLimiter(pool, key)
	if err != nil {
		t.Fatal(err)
	}

	limiterB, err := ratelimit.NewPostgresLimiter(pool, key)
	if err != nil {
		t.Fatal(err)
	}

	scope := "test:" + uuid.NewString()
	identity := "192.0.2.10"

	ctx := context.Background()

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		_, err := pool.Exec(
			cleanupCtx,
			"DELETE FROM auth_rate_limits WHERE scope = $1",
			scope,
		)
		if err != nil {
			t.Errorf("cleanup rate limit: %v", err)
		}
	})

	first, err := limiterA.Allow(
		ctx, scope, identity, 2, time.Minute,
	)
	if err != nil || !first.Allowed {
		t.Fatalf("first request should pass: %v", err)
	}

	second, err := limiterB.Allow(
		ctx, scope, identity, 2, time.Minute,
	)
	if err != nil || !second.Allowed {
		t.Fatalf("second request should pass: %v", err)
	}

	third, err := limiterA.Allow(
		ctx, scope, identity, 2, time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}

	if third.Allowed || third.RetryAfter <= 0 {
		t.Fatal("third request must be rate limited")
	}

	// A different identity must have an independent counter.
	other, err := limiterB.Allow(
		ctx, scope, "192.0.2.11", 2, time.Minute,
	)
	if err != nil || !other.Allowed {
		t.Fatal("different identity should be allowed")
	}
}

func TestRateLimiterConcurrentRequests(t *testing.T) {
	pool := openTestDB(t)

	limiter, err := ratelimit.NewPostgresLimiter(
		pool,
		bytes.Repeat([]byte{0x24}, 32),
	)
	if err != nil {
		t.Fatal(err)
	}

	scope := "test:" + uuid.NewString()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		_, err := pool.Exec(
			ctx,
			"DELETE FROM auth_rate_limits WHERE scope = $1",
			scope,
		)
		if err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	const totalRequests = 12
	const allowedLimit = 3

	type result struct {
		allowed bool
		err     error
	}

	results := make(chan result, totalRequests)

	var wg sync.WaitGroup

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			decision, err := limiter.Allow(
				context.Background(),
				scope,
				"192.0.2.25",
				allowedLimit,
				time.Minute,
			)

			results <- result{
				allowed: decision.Allowed,
				err:     err,
			}
		}()
	}

	wg.Wait()
	close(results)

	allowed := 0

	for current := range results {
		if current.err != nil {
			t.Fatal(current.err)
		}

		if current.allowed {
			allowed++
		}
	}

	if allowed != allowedLimit {
		t.Fatalf(
			"expected %d allowed requests, got %d",
			allowedLimit, allowed,
		)
	}
}
