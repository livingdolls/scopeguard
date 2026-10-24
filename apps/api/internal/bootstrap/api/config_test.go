package api

import (
	"bytes"
	"encoding/base64"
	"slices"
	"strings"
	"testing"
)

func setTestAPIEnvironment(t *testing.T) {
	t.Helper()
	for name, value := range map[string]string{
		"DATABASE_URL":            "postgres://scopeguard@localhost/scopeguard",
		"HTTP_ADDR":               "127.0.0.1:9000",
		"DB_MAX_CONNECTIONS":      "7",
		"JWT_ACCESS_SECRET_B64":   base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x51}, 32)),
		"JWT_ISSUER":              "scopeguard-test",
		"JWT_AUDIENCE":            "scopeguard-api-test",
		"EMAIL_OUTBOX_KEY_B64":    base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x37}, 32)),
		"EMAIL_OUTBOX_KEY_ID":     "test-v1",
		"AUTH_RATE_LIMIT_KEY_B64": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x29}, 32)),
		"TRUSTED_PROXY_CIDRS":     "",
	} {
		t.Setenv(name, value)
	}
}

func TestLoadConfig(t *testing.T) {
	setTestAPIEnvironment(t)
	t.Setenv("TRUSTED_PROXY_CIDRS", "127.0.0.1, 10.0.0.0/8")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HTTPAddr != "127.0.0.1:9000" || cfg.DBMaxConnections != 7 ||
		cfg.DatabaseURL != "postgres://scopeguard@localhost/scopeguard" {
		t.Fatal("API configuration must include the shared HTTP and database settings")
	}
	if !bytes.Equal(cfg.Auth.JWTSecret, bytes.Repeat([]byte{0x51}, 32)) ||
		!bytes.Equal(cfg.Auth.OutboxKey, bytes.Repeat([]byte{0x37}, 32)) ||
		!bytes.Equal(cfg.Auth.RateLimitKey, bytes.Repeat([]byte{0x29}, 32)) {
		t.Fatal("authentication keys must be decoded independently")
	}
	if cfg.Auth.JWTIssuer != "scopeguard-test" || cfg.Auth.JWTAudience != "scopeguard-api-test" ||
		cfg.Auth.OutboxKeyID != "test-v1" {
		t.Fatal("missing authentication configuration")
	}
	if !slices.Equal(cfg.Auth.TrustedProxies, []string{"127.0.0.1", "10.0.0.0/8"}) {
		t.Fatalf("unexpected trusted proxies: %v", cfg.Auth.TrustedProxies)
	}
}

func TestLoadConfigRejectsInvalidEnvironment(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{"missing database URL", "DATABASE_URL", ""},
		{"missing JWT key", "JWT_ACCESS_SECRET_B64", ""},
		{"invalid outbox key encoding", "EMAIL_OUTBOX_KEY_B64", "invalid-base64"},
		{"short rate limit key", "AUTH_RATE_LIMIT_KEY_B64", base64.StdEncoding.EncodeToString([]byte("short"))},
		{"missing issuer", "JWT_ISSUER", ""},
		{"missing audience", "JWT_AUDIENCE", ""},
		{"missing outbox key ID", "EMAIL_OUTBOX_KEY_ID", ""},
		{"empty proxy list entry", "TRUSTED_PROXY_CIDRS", "127.0.0.1,"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setTestAPIEnvironment(t)
			t.Setenv(tt.key, tt.value)
			if _, err := LoadConfig(); err == nil {
				t.Fatalf("expected invalid %s to reject startup", tt.key)
			}
		})
	}
}

func TestLoadConfigDefaultsToNoTrustedProxies(t *testing.T) {
	setTestAPIEnvironment(t)
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Auth.TrustedProxies) != 0 {
		t.Fatal("proxy headers must be untrusted by default")
	}
}

func TestNewRouterRejectsInvalidRoutingConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name:    "invalid trusted proxy",
			cfg:     Config{Auth: AuthConfig{TrustedProxies: []string{"invalid-cidr"}}},
			wantErr: "set trusted proxies",
		},
		{
			name:    "oversized rate limit scope",
			cfg:     Config{RateLimitScopePrefix: strings.Repeat("x", 53)},
			wantErr: "rate limit scope prefix",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewRouter(nil, tt.cfg, nil)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected %q error, got %v", tt.wantErr, err)
			}
		})
	}
}
