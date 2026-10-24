package ratelimit

import (
	"context"
	"time"
)

type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
}

type Limiter interface {
	Allow(
		ctx context.Context,
		scope string,
		identity string,
		limit int,
		window time.Duration,
	) (Decision, error)
}
