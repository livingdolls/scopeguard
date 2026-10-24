package ratelimit

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresLimiter struct {
	pool *pgxpool.Pool
	key  []byte
}

var _ Limiter = (*PostgresLimiter)(nil)

func NewPostgresLimiter(pool *pgxpool.Pool, key []byte) (*PostgresLimiter, error) {
	if pool == nil || len(key) < 32 {
		return nil, errors.New("rate limiter requires pool and 32 bute HMAC key")
	}
	return &PostgresLimiter{
		pool: pool,
		key:  key,
	}, nil
}

func (l *PostgresLimiter) digest(scope string, identity string) string {
	mac := hmac.New(sha256.New, l.key)

	mac.Write([]byte(scope))
	mac.Write([]byte{0})
	mac.Write([]byte(identity))

	return hex.EncodeToString(mac.Sum(nil))
}

func (l *PostgresLimiter) Allow(ctx context.Context, scope string, identity string, limit int, window time.Duration) (Decision, error) {
	if scope == "" || len(scope) > 64 ||
		identity == "" ||
		limit < 1 ||
		window < time.Second ||
		window > 24*time.Hour ||
		window%time.Second != 0 {
		return Decision{}, errors.New("invalid rate limit parameters")
	}

	const query = `
		INSERT INTO auth_rate_limits AS r (
		    scope,
		    identity_hash,
		    hits,
			reset_at
		)
		VALUES (
		    $1, $2, 1,
		    NOW() + ($3::bigint * INTERVAL '1 second')
		)
		ON CONFLICT (scope, identity_hash)
		DO UPDATE SET
		    hits = CASE
				WHEN r.reset_at <= NOW() THEN 1
				ELSE r.hits + 1
			END,
			reset_at = CASE
				WHEN r.reset_at <= NOW()
					THEN NOW() + ($3::bigint * INTERVAL '1 second')
				ELSE r.reset_at
			END
		RETURNING hits, reset_at
	`

	var hits int
	var resetAt time.Time

	err := l.pool.QueryRow(
		ctx,
		query,
		scope,
		l.digest(scope, identity),
		int64(window/time.Second),
	).Scan(&hits, &resetAt)

	if err != nil {
		return Decision{}, fmt.Errorf("evaluating rate limits: %w", err)
	}

	if hits <= limit {
		return Decision{Allowed: true}, nil
	}

	retry := time.Until(resetAt)
	if retry < time.Second {
		retry = time.Second
	}

	return Decision{Allowed: false, RetryAfter: retry}, nil
}
