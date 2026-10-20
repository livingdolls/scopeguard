//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	passwordadapter "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/adapters/outbound/password"
	identitypostgres "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/adapters/outbound/postgres"
	tokenadapter "github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/adapters/outbound/token"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
)

func TestLoginUserIntegration(t *testing.T) {
	pool := openTestDB(t)

	repo := identitypostgres.NewUserRepository(pool)
	hasher := passwordadapter.NewArgon2idHasher()

	registerService := application.NewRegisterUserService(
		repo, hasher,
	)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		20*time.Second,
	)
	defer cancel()

	password := "correct horse battery staple"

	registered, err := registerService.Execute(
		ctx,
		application.RegisterUserInput{
			Email:    "login-" + uuid.NewString() + "@example.test",
			Password: password,
		},
	)
	if err != nil {
		t.Fatalf("register test user: %v", err)
	}

	// Helper from M1.5.3.2.
	cleanupRegisteredUser(t, pool, registered.ID)

	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}

	tokenService, err := tokenadapter.NewJWTAccessToken(
		secret,
		"scopeguard",
		"scopeguard-api",
		15*time.Minute,
	)
	if err != nil {
		t.Fatal(err)
	}

	dummyHash, err := hasher.Hash(
		"scopeguard dummy password for verification",
	)
	if err != nil {
		t.Fatal(err)
	}

	loginService, err := application.NewLoginUserService(
		repo,
		hasher,
		tokenService,
		dummyHash,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Unverified account must not receive an access token.
	_, err = loginService.Execute(
		ctx,
		application.LoginUserInput{
			Email:    registered.Email,
			Password: password,
		},
	)

	if !errors.Is(err, application.ErrInvalidCredentials) {
		t.Fatalf("pending account must not log in: %v", err)
	}

	outboxRepo, sealer := newVerificationOutboxForTest(
		t, pool,
	)
	issuer := application.NewIssueEmailVerificationService(outboxRepo)
	verifier := application.NewVerifyEmailService(repo)

	eventID, err := issuer.Execute(
		ctx,
		registered.ID,
	)
	if err != nil {
		t.Fatalf("issue verification token: %v", err)
	}

	verificationToken := readOutboxToken(
		t, pool, sealer, eventID,
	)

	if err := verifier.Execute(
		ctx,
		verificationToken,
	); err != nil {
		t.Fatalf("verify email: %v", err)
	}

	result, err := loginService.Execute(
		ctx,
		application.LoginUserInput{
			Email:    registered.Email,
			Password: password,
		},
	)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	verifiedUserID, err := tokenService.Verify(
		result.AccessToken,
	)
	if err != nil {
		t.Fatalf("verify access token: %v", err)
	}

	if verifiedUserID != registered.ID {
		t.Fatal("JWT subject does not match registered user")
	}

	_, err = loginService.Execute(
		ctx,
		application.LoginUserInput{
			Email:    registered.Email,
			Password: "wrong password",
		},
	)

	if !errors.Is(err, application.ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}

	_, err = pool.Exec(
		ctx,
		`UPDATE users
         SET status = 'suspended'
         WHERE id = $1`,
		registered.ID,
	)
	if err != nil {
		t.Fatal(err)
	}

	allowed, err := repo.CanAuthenticate(
		ctx,
		registered.ID,
	)
	if err != nil {
		t.Fatal(err)
	}

	if allowed {
		t.Fatal("suspended user must be denied access")
	}
}
