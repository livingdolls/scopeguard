//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	bootstrapapi "github.com/livingdolls/scopeguard/apps/api/internal/bootstrap/api"
	encryptionadapter "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/adapters/outbound/encryption"
	tokenadapter "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/adapters/outbound/token"
)

const authHTTPPassword = "correct horse battery staple"

type authHTTPFixture struct {
	pool         *pgxpool.Pool
	routerPool   *pgxpool.Pool
	router       *gin.Engine
	sealer       *encryptionadapter.AESGCM
	jwtService   *tokenadapter.JWTAccessToken
	rateLimitKey []byte
}

type authHTTPUser struct {
	id                uuid.UUID
	email             string
	verificationToken string
}

func newAuthHTTPFixture(t *testing.T) *authHTTPFixture {
	t.Helper()

	pool := openTestDB(t)
	cfg := bootstrapapi.Config{
		Auth: bootstrapapi.AuthConfig{
			JWTSecret:    bytes.Repeat([]byte{0x51}, 32),
			JWTIssuer:    "scopeguard-test",
			JWTAudience:  "scopeguard-api-test",
			OutboxKey:    bytes.Repeat([]byte{0x37}, 32),
			OutboxKeyID:  "http-flow-v1",
			RateLimitKey: bytes.Repeat([]byte{0x29}, 32),
		},
		RateLimitScopePrefix: "test:http:" + uuid.NewString(),
	}
	// Keep a separate cleanup pool usable when failure tests close the router pool.
	routerPool := openTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router, err := bootstrapapi.NewRouter(routerPool, cfg, logger)
	if err != nil {
		t.Fatalf("create production API router: %v", err)
	}

	// These adapters are only for inspecting the encrypted outbox and issued JWT.
	sealer, err := encryptionadapter.NewAESGCM(cfg.Auth.OutboxKey, cfg.Auth.OutboxKeyID)
	if err != nil {
		t.Fatal(err)
	}
	jwtService, err := tokenadapter.NewJWTAccessToken(
		cfg.Auth.JWTSecret, cfg.Auth.JWTIssuer, cfg.Auth.JWTAudience, 15*time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		scopes := []string{
			cfg.RateLimitScopePrefix + ":register:ip",
			cfg.RateLimitScopePrefix + ":login:ip",
			cfg.RateLimitScopePrefix + ":verify:ip",
		}
		if _, err := pool.Exec(ctx, "DELETE FROM auth_rate_limits WHERE scope = ANY($1)", scopes); err != nil {
			t.Errorf("cleanup HTTP rate limits: %v", err)
		}
	})

	return &authHTTPFixture{
		pool: pool, routerPool: routerPool, router: router,
		sealer: sealer, jwtService: jwtService, rateLimitKey: cfg.Auth.RateLimitKey,
	}
}

func (f *authHTTPFixture) request(t *testing.T, method, path, body, accessToken string) *httptest.ResponseRecorder {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
	// Keep the same socket IP for every request, with proxy headers untrusted.
	req.RemoteAddr = "192.0.2.80:45000"
	req.Header.Set("Content-Type", "application/json")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	response := httptest.NewRecorder()
	f.router.ServeHTTP(response, req)
	return response
}

func requireAuthHTTPStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("expected HTTP %d, got %d: %s", want, response.Code, response.Body.String())
	}
}

func decodeAuthHTTPJSON(t *testing.T, response *httptest.ResponseRecorder, output any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), output); err != nil {
		t.Fatalf("decode HTTP response: %v", err)
	}
}

func (f *authHTTPFixture) register(t *testing.T) authHTTPUser {
	t.Helper()

	user := authHTTPUser{email: "http-flow-" + uuid.NewString() + "@example.test"}
	response := f.request(t, http.MethodPost, "/v1/auth/register",
		fmt.Sprintf(`{"email":%q,"password":%q}`, user.email, authHTTPPassword), "")
	requireAuthHTTPStatus(t, response, http.StatusAccepted)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := f.pool.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", user.email).Scan(&user.id); err != nil {
		t.Fatalf("find registered HTTP user: %v", err)
	}
	cleanupRegisteredUser(t, f.pool, user.id)

	// The production email scope is fixed; remove only this unique email's counter.
	mac := hmac.New(sha256.New, f.rateLimitKey)
	mac.Write([]byte("auth:login:email"))
	mac.Write([]byte{0})
	mac.Write([]byte(user.email))
	emailHash := hex.EncodeToString(mac.Sum(nil))
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, err := f.pool.Exec(cleanupCtx,
			"DELETE FROM auth_rate_limits WHERE scope = $1 AND identity_hash = $2",
			"auth:login:email", emailHash)
		if err != nil {
			t.Errorf("cleanup HTTP email rate limit: %v", err)
		}
	})

	// Decrypt the outbox only in this integration helper, never through HTTP.
	eventID := registrationOutboxID(t, f.pool, user.id)
	user.verificationToken = readOutboxToken(t, f.pool, f.sealer, eventID)
	return user
}

func TestAuthHTTPFlow(t *testing.T) {
	f := newAuthHTTPFixture(t)
	user := f.register(t)
	credentials := fmt.Sprintf(`{"email":%q,"password":%q}`, user.email, authHTTPPassword)

	t.Run("login before email verification", func(t *testing.T) {
		response := f.request(t, http.MethodPost, "/v1/auth/login", credentials, "")
		requireAuthHTTPStatus(t, response, http.StatusUnauthorized)
	})

	verificationBody := fmt.Sprintf(`{"token":%q}`, user.verificationToken)
	response := f.request(t, http.MethodPost, "/v1/auth/verify-email", verificationBody, "")
	requireAuthHTTPStatus(t, response, http.StatusNoContent)
	if response.Body.Len() != 0 {
		t.Fatal("email verification must return no response body")
	}

	t.Run("verification token cannot be reused", func(t *testing.T) {
		response := f.request(t, http.MethodPost, "/v1/auth/verify-email", verificationBody, "")
		requireAuthHTTPStatus(t, response, http.StatusBadRequest)
	})

	t.Run("wrong password", func(t *testing.T) {
		response := f.request(t, http.MethodPost, "/v1/auth/login",
			fmt.Sprintf(`{"email":%q,"password":"wrong password"}`, user.email), "")
		requireAuthHTTPStatus(t, response, http.StatusUnauthorized)
	})

	response = f.request(t, http.MethodPost, "/v1/auth/login", credentials, "")
	requireAuthHTTPStatus(t, response, http.StatusOK)
	var login struct {
		AccessToken string    `json:"access_token"`
		TokenType   string    `json:"token_type"`
		ExpiresAt   time.Time `json:"expires_at"`
	}
	decodeAuthHTTPJSON(t, response, &login)
	if login.AccessToken == "" || login.TokenType != "Bearer" || !login.ExpiresAt.After(time.Now()) {
		t.Fatal("login must return a Bearer access token with a future expiration")
	}
	jwtUserID, err := f.jwtService.Verify(login.AccessToken)
	if err != nil || jwtUserID != user.id {
		t.Fatalf("login JWT must identify the registered user: id=%s, err=%v", jwtUserID, err)
	}

	response = f.request(t, http.MethodGet, "/v1/auth/me", "", login.AccessToken)
	requireAuthHTTPStatus(t, response, http.StatusOK)
	var profile struct {
		ID    uuid.UUID `json:"id"`
		Email string    `json:"email"`
	}
	decodeAuthHTTPJSON(t, response, &profile)
	if profile.ID != user.id || profile.Email != user.email {
		t.Fatalf("incorrect current user profile: %+v", profile)
	}

	t.Run("missing JWT", func(t *testing.T) {
		response := f.request(t, http.MethodGet, "/v1/auth/me", "", "")
		requireAuthHTTPStatus(t, response, http.StatusUnauthorized)
	})

	t.Run("invalid JWT signature", func(t *testing.T) {
		parts := strings.Split(login.AccessToken, ".")
		signature, err := base64.RawURLEncoding.DecodeString(parts[2])
		if err != nil || len(signature) == 0 {
			t.Fatal("login JWT has no valid signature encoding")
		}
		signature[0] ^= 0xff
		parts[2] = base64.RawURLEncoding.EncodeToString(signature)
		response := f.request(t, http.MethodGet, "/v1/auth/me", "", strings.Join(parts, "."))
		requireAuthHTTPStatus(t, response, http.StatusUnauthorized)
	})

	slug := uniqueOrganizationSlug()
	cleanupOrganization(t, f.pool, slug)
	response = f.request(t, http.MethodPost, "/v1/organizations",
		fmt.Sprintf(`{"name":"Integration Test Organization","slug":%q}`, slug), login.AccessToken)
	requireAuthHTTPStatus(t, response, http.StatusCreated)
	var organization struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
		Slug string    `json:"slug"`
	}
	decodeAuthHTTPJSON(t, response, &organization)
	if organization.ID == uuid.Nil || organization.Name != "Integration Test Organization" || organization.Slug != slug {
		t.Fatalf("incorrect organization response: %+v", organization)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var ownerID uuid.UUID
	var role string
	err = f.pool.QueryRow(ctx,
		"SELECT user_id, role FROM organization_memberships WHERE organization_id = $1",
		organization.ID).Scan(&ownerID, &role)
	if err != nil {
		t.Fatalf("find organization owner membership: %v", err)
	}
	if ownerID != jwtUserID || role != "owner" {
		t.Fatalf("expected JWT user %s as owner, got %s with role %q", jwtUserID, ownerID, role)
	}

	t.Run("suspension revokes protected access", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := f.pool.Exec(ctx, "UPDATE users SET status = 'suspended' WHERE id = $1", user.id); err != nil {
			t.Fatal(err)
		}
		response := f.request(t, http.MethodGet, "/v1/auth/me", "", login.AccessToken)
		requireAuthHTTPStatus(t, response, http.StatusUnauthorized)
		response = f.request(t, http.MethodPost, "/v1/organizations",
			fmt.Sprintf(`{"name":"Suspended User Organization","slug":%q}`, slug), login.AccessToken)
		requireAuthHTTPStatus(t, response, http.StatusUnauthorized)
	})
}

func TestAuthHTTPIPRateLimits(t *testing.T) {
	f := newAuthHTTPFixture(t)
	tests := []struct {
		name       string
		path       string
		body       string
		limit      int
		wantStatus int
	}{
		{"register", "/v1/auth/register", `{}`, 5, http.StatusBadRequest},
		{"login", "/v1/auth/login", `{"email":"not-an-email","password":"wrong password"}`, 20, http.StatusUnauthorized},
		{"verify email", "/v1/auth/verify-email", `{"token":"invalid"}`, 10, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < tt.limit; i++ {
				response := f.request(t, http.MethodPost, tt.path, tt.body, "")
				requireAuthHTTPStatus(t, response, tt.wantStatus)
			}
			response := f.request(t, http.MethodPost, tt.path, tt.body, "")
			requireAuthHTTPStatus(t, response, http.StatusTooManyRequests)
			retryAfter, err := strconv.Atoi(response.Header().Get("Retry-After"))
			if err != nil || retryAfter < 1 {
				t.Fatal("rate limited response must include a positive Retry-After")
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("rate limited response must not be cached")
			}
		})
	}
}

func TestAuthHTTPEmailRateLimit(t *testing.T) {
	f := newAuthHTTPFixture(t)
	user := f.register(t)
	body := fmt.Sprintf(`{"email":%q,"password":"wrong password"}`, user.email)
	for i := 0; i < 5; i++ {
		response := f.request(t, http.MethodPost, "/v1/auth/login", body, "")
		requireAuthHTTPStatus(t, response, http.StatusUnauthorized)
	}
	response := f.request(t, http.MethodPost, "/v1/auth/login", body, "")
	requireAuthHTTPStatus(t, response, http.StatusTooManyRequests)
}

func TestAuthHTTPRateLimiterUnavailable(t *testing.T) {
	f := newAuthHTTPFixture(t)
	// The real limiter's pool fails while the separate cleanup pool remains usable.
	f.routerPool.Close()
	for _, path := range []string{"/v1/auth/register", "/v1/auth/login", "/v1/auth/verify-email"} {
		t.Run(path, func(t *testing.T) {
			response := f.request(t, http.MethodPost, path, `{}`, "")
			requireAuthHTTPStatus(t, response, http.StatusServiceUnavailable)
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("unavailable response must not be cached")
			}
		})
	}
}
