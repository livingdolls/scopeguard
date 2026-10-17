package token

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
)

func newTestTokenService(t *testing.T) *JWTAccessToken {
	t.Helper()

	service, err := NewJWTAccessToken(
		[]byte(strings.Repeat("x", 32)),
		"scopeguard",
		"scopeguard-api",
		15*time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}

	return service
}

func TestJWTAccessTokenIssueAndVerify(t *testing.T) {
	service := newTestTokenService(t)
	userID := uuid.New()

	raw, expiresAt, err := service.Issue(userID)
	if err != nil {
		t.Fatal(err)
	}

	if raw == "" || expiresAt.IsZero() {
		t.Fatal("missing access token or expiration")
	}

	verifiedID, err := service.Verify(raw)
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}

	if verifiedID != userID {
		t.Fatal("verified user ID differs from original")
	}
}

func TestJWTAccessTokenExpired(t *testing.T) {
	service := newTestTokenService(t)

	fixedTime := time.Date(
		2026, 10, 17, 9, 0, 0, 0, time.UTC,
	)

	service.now = func() time.Time {
		return fixedTime
	}

	raw, _, err := service.Issue(uuid.New())
	if err != nil {
		t.Fatal(err)
	}

	// Advance the clock beyond expiration.
	service.now = func() time.Time {
		return fixedTime.Add(16 * time.Minute)
	}

	_, err = service.Verify(raw)

	if !errors.Is(err, application.ErrInvalidAccessToken) {
		t.Fatalf("expected expired token rejection, got %v", err)
	}
}

func TestJWTAccessTokenWrongSecret(t *testing.T) {
	issuer := newTestTokenService(t)
	userID := uuid.New()

	raw, _, err := issuer.Issue(userID)
	if err != nil {
		t.Fatal(err)
	}

	verifier, err := NewJWTAccessToken(
		[]byte(strings.Repeat("y", 32)),
		"scopeguard",
		"scopeguard-api",
		15*time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = verifier.Verify(raw)

	if !errors.Is(err, application.ErrInvalidAccessToken) {
		t.Fatalf("expected signature rejection, got %v", err)
	}
}

func TestJWTAccessTokenWrongAudience(t *testing.T) {
	issuer := newTestTokenService(t)

	raw, _, err := issuer.Issue(uuid.New())
	if err != nil {
		t.Fatal(err)
	}

	verifier, err := NewJWTAccessToken(
		[]byte(strings.Repeat("x", 32)),
		"scopeguard",
		"another-api",
		15*time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = verifier.Verify(raw)

	if !errors.Is(err, application.ErrInvalidAccessToken) {
		t.Fatalf("expected audience rejection, got %v", err)
	}
}

func TestJWTAccessTokenRejectsInvalidInput(t *testing.T) {
	service := newTestTokenService(t)

	_, _, err := service.Issue(uuid.Nil)
	if err == nil {
		t.Fatal("expected invalid user ID error")
	}

	_, err = service.Verify("invalid.jwt.token")
	if !errors.Is(err, application.ErrInvalidAccessToken) {
		t.Fatalf("expected invalid token error, got %v", err)
	}

	_, err = NewJWTAccessToken(
		[]byte("weak-key"),
		"scopeguard",
		"scopeguard-api",
		15*time.Minute,
	)
	if err == nil {
		t.Fatal("expected short signing key rejection")
	}
}
