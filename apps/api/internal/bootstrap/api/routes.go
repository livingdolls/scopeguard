package api

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/livingdolls/scopeguard/apps/api/internal/platform/httpapi"
	"github.com/livingdolls/scopeguard/apps/api/internal/platform/ratelimit"
)

// NewRouter wires the production adapters and routes using the caller's pool.
// The caller retains ownership of the pool.
func NewRouter(pool *pgxpool.Pool, cfg Config, logger *slog.Logger) (*gin.Engine, error) {
	if logger == nil {
		logger = slog.Default()
	}

	router := httpapi.NewRouter()
	if err := router.SetTrustedProxies(cfg.Auth.TrustedProxies); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}
	rules, err := authIPRateLimitRules(cfg.RateLimitScopePrefix)
	if err != nil {
		return nil, err
	}
	limiter, err := ratelimit.NewPostgresLimiter(pool, cfg.Auth.RateLimitKey)
	if err != nil {
		return nil, fmt.Errorf("create rate limiter: %w", err)
	}
	identity, err := newIdentity(pool, cfg.Auth, limiter, logger)
	if err != nil {
		return nil, fmt.Errorf("initialize identity module: %w", err)
	}
	organization := newOrganization(pool, logger)
	system := newSystem(pool, logger)

	system.health.RegisterRoutes(router)
	system.readiness.RegisterRoutes(router)

	public := router.Group("/v1")
	public.Use(ratelimit.LimitIP(limiter, rules))
	identity.handler.RegisterPublicRoutes(public)

	protected := router.Group("/v1")
	protected.Use(identity.requireAuth)
	identity.handler.RegisterProtectedRoutes(protected)
	organization.RegisterRoutes(protected)

	return router, nil
}

func authIPRateLimitRules(prefix string) (map[string]ratelimit.Rule, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "auth"
	}
	if len(prefix)+len(":register:ip") > 64 {
		return nil, fmt.Errorf("rate limit scope prefix must be at most %d bytes", 64-len(":register:ip"))
	}

	return map[string]ratelimit.Rule{
		"POST /v1/auth/register": {
			Scope: prefix + ":register:ip", Limit: 5, Window: 15 * time.Minute,
		},
		"POST /v1/auth/login": {
			Scope: prefix + ":login:ip", Limit: 20, Window: 5 * time.Minute,
		},
		"POST /v1/auth/verify-email": {
			Scope: prefix + ":verify:ip", Limit: 10, Window: 5 * time.Minute,
		},
	}, nil
}
