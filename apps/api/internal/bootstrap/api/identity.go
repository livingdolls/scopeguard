package api

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	identityhttp "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/adapters/inbound/http"
	identityencryption "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/adapters/outbound/encryption"
	identitypassword "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/adapters/outbound/password"
	identitypostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/adapters/outbound/postgres"
	identitytoken "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/adapters/outbound/token"
	identityapp "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/platform/ratelimit"
)

type identityModule struct {
	handler     *identityhttp.Handler
	requireAuth gin.HandlerFunc
}

func newIdentity(pool *pgxpool.Pool, cfg AuthConfig, limiter ratelimit.Limiter, logger *slog.Logger) (identityModule, error) {
	userRepo := identitypostgres.NewUserRepository(pool)
	hasher := identitypassword.NewArgon2idHasher()

	sealer, err := identityencryption.NewAESGCM(cfg.OutboxKey, cfg.OutboxKeyID)
	if err != nil {
		return identityModule{}, fmt.Errorf("create outbox sealer: %w", err)
	}
	accessToken, err := identitytoken.NewJWTAccessToken(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTAudience, 15*time.Minute)
	if err != nil {
		return identityModule{}, fmt.Errorf("create JWT service: %w", err)
	}
	dummyHash, err := hasher.Hash("scopeguard dummy verification passphrase")
	if err != nil {
		return identityModule{}, fmt.Errorf("create dummy password hash: %w", err)
	}
	loginService, err := identityapp.NewLoginUserService(userRepo, hasher, accessToken, dummyHash)
	if err != nil {
		return identityModule{}, fmt.Errorf("create login service: %w", err)
	}

	return identityModule{
		handler: identityhttp.NewHandler(
			identityapp.NewRegisterUserService(userRepo, hasher, sealer),
			identityapp.NewVerifyEmailService(userRepo),
			loginService,
			identityapp.NewGetCurrentUserService(userRepo),
			limiter,
			logger,
		),
		requireAuth: identityhttp.RequireBearerAuth(accessToken, userRepo, logger),
	}, nil
}
