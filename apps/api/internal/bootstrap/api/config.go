package api

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	platformconfig "github.com/livingdolls/scopeguard/apps/api/internal/platform/config"
)

type Config struct {
	platformconfig.Config
	Auth AuthConfig

	// RateLimitScopePrefix namespaces public IP counters; empty uses "auth".
	RateLimitScopePrefix string
}

type AuthConfig struct {
	JWTSecret   []byte
	JWTIssuer   string
	JWTAudience string

	OutboxKey   []byte
	OutboxKeyID string

	RateLimitKey   []byte
	TrustedProxies []string
}

func LoadConfig() (Config, error) {
	base, err := platformconfig.Load()
	if err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}

	auth, err := loadAuthConfig()
	if err != nil {
		return Config{}, fmt.Errorf("load auth config: %w", err)
	}

	return Config{Config: base, Auth: auth}, nil
}

func requiredSecret(name string) ([]byte, error) {
	value := strings.TrimSpace(os.Getenv(name))

	decoded, err := base64.StdEncoding.DecodeString(value)

	if err != nil || len(decoded) != 32 {
		return nil, fmt.Errorf("%s must contain a Base64-encoded 32-byte key", name)
	}

	return decoded, nil
}

func loadAuthConfig() (AuthConfig, error) {
	jwtSecret, err := requiredSecret("JWT_ACCESS_SECRET_B64")
	if err != nil {
		return AuthConfig{}, err
	}

	outboxKey, err := requiredSecret("EMAIL_OUTBOX_KEY_B64")
	if err != nil {
		return AuthConfig{}, err
	}

	rateKey, err := requiredSecret("AUTH_RATE_LIMIT_KEY_B64")
	if err != nil {
		return AuthConfig{}, err
	}

	issuer := strings.TrimSpace(os.Getenv("JWT_ISSUER"))
	audience := strings.TrimSpace(os.Getenv("JWT_AUDIENCE"))
	keyID := strings.TrimSpace(os.Getenv("EMAIL_OUTBOX_KEY_ID"))

	if issuer == "" || audience == "" || keyID == "" {
		return AuthConfig{}, fmt.Errorf("authentication configuration is incomplete")
	}

	var proxies []string

	rawProxies := strings.TrimSpace(os.Getenv("TRUSTED_PROXY_CIDRS"))

	if rawProxies != "" {
		for _, value := range strings.Split(rawProxies, ",") {
			cidr := strings.TrimSpace(value)

			if cidr == "" {
				return AuthConfig{}, fmt.Errorf("trusted proxy cidr is empty")
			}

			proxies = append(proxies, cidr)
		}
	}

	return AuthConfig{
		JWTSecret:   jwtSecret,
		JWTIssuer:   issuer,
		JWTAudience: audience,

		OutboxKey:   outboxKey,
		OutboxKeyID: keyID,

		RateLimitKey:   rateKey,
		TrustedProxies: proxies,
	}, nil
}
