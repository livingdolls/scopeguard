package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/domain"
)

type loginRepoStub struct {
	credential LoginCredential
	err        error
	calls      int
}

func (s *loginRepoStub) FindByEmail(
	ctx context.Context,
	email string,
) (LoginCredential, error) {
	s.calls++
	return s.credential, s.err
}

type loginHasherStub struct {
	calls int
	err   error
}

func (s *loginHasherStub) Hash(
	password string,
) (string, error) {
	return "unused", nil
}

func (s *loginHasherStub) Verify(
	password string,
	encodedHash string,
) (bool, error) {
	s.calls++

	if s.err != nil {
		return false, s.err
	}

	return password == "correct horse battery staple" &&
		encodedHash == "stored-test-hash", nil
}

type loginTokenStub struct {
	issuedUser uuid.UUID
	calls      int
}

func (s *loginTokenStub) Issue(
	userID uuid.UUID,
) (string, time.Time, error) {
	s.calls++
	s.issuedUser = userID

	return "test-access-token",
		time.Now().Add(15 * time.Minute), nil
}

func TestLoginUserSuccess(t *testing.T) {
	userID := uuid.New()

	repo := &loginRepoStub{
		credential: LoginCredential{
			UserID:       userID,
			PasswordHash: "stored-test-hash",
			Status:       domain.AccountActive,
		},
	}
	hasher := &loginHasherStub{}
	tokens := &loginTokenStub{}

	service, err := NewLoginUserService(
		repo, hasher, tokens, "dummy-test-hash",
	)
	if err != nil {
		t.Fatal(err)
	}

	output, err := service.Execute(
		context.Background(),
		LoginUserInput{
			Email:    "  USER@EXAMPLE.COM  ",
			Password: "correct horse battery staple",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if output.AccessToken != "test-access-token" {
		t.Fatal("unexpected token")
	}

	if output.TokenType != "Bearer" {
		t.Fatal("unexpected token type")
	}

	if tokens.issuedUser != userID || tokens.calls != 1 {
		t.Fatal("token issued for incorrect user")
	}

	if repo.calls != 1 || hasher.calls != 1 {
		t.Fatal("unexpected repository or hasher calls")
	}
}

func TestLoginUserInvalidPassword(t *testing.T) {
	repo := &loginRepoStub{
		credential: LoginCredential{
			UserID:       uuid.New(),
			PasswordHash: "stored-test-hash",
			Status:       domain.AccountActive,
		},
	}

	hasher := &loginHasherStub{}
	tokens := &loginTokenStub{}

	service, err := NewLoginUserService(
		repo, hasher, tokens, "dummy-test-hash",
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Execute(
		context.Background(),
		LoginUserInput{
			Email:    "user@example.com",
			Password: "wrong password",
		},
	)

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}

	if tokens.calls != 0 {
		t.Fatal("token must not be issued")
	}
}

func TestLoginUserInactiveAccount(t *testing.T) {
	statuses := []domain.AccountStatus{
		domain.AccountPendingVerification,
		domain.AccountSuspended,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			repo := &loginRepoStub{
				credential: LoginCredential{
					UserID:       uuid.New(),
					PasswordHash: "stored-test-hash",
					Status:       status,
				},
			}

			hasher := &loginHasherStub{}
			tokens := &loginTokenStub{}

			service, err := NewLoginUserService(
				repo, hasher, tokens, "dummy-test-hash",
			)
			if err != nil {
				t.Fatal(err)
			}

			_, err = service.Execute(
				context.Background(),
				LoginUserInput{
					Email:    "user@example.com",
					Password: "correct horse battery staple",
				},
			)

			if !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("expected invalid credentials, got %v", err)
			}

			if hasher.calls != 1 {
				t.Fatal("password must be verified")
			}

			if tokens.calls != 0 {
				t.Fatal("inactive account must not receive JWT")
			}
		})
	}
}

func TestLoginUserUnknownEmail(t *testing.T) {
	repo := &loginRepoStub{err: ErrUserNotFound}
	hasher := &loginHasherStub{}
	tokens := &loginTokenStub{}

	service, err := NewLoginUserService(
		repo, hasher, tokens, "dummy-test-hash",
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Execute(
		context.Background(),
		LoginUserInput{
			Email:    "missing@example.com",
			Password: "correct horse battery staple",
		},
	)

	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}

	if hasher.calls != 1 {
		t.Fatal("dummy password verification was not executed")
	}

	if tokens.calls != 0 {
		t.Fatal("unknown user must not receive a token")
	}
}

func TestLoginUserCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	repo := &loginRepoStub{}
	hasher := &loginHasherStub{}
	tokens := &loginTokenStub{}

	service, err := NewLoginUserService(
		repo, hasher, tokens, "dummy-test-hash",
	)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Execute(
		ctx,
		LoginUserInput{
			Email:    "user@example.com",
			Password: "correct horse battery staple",
		},
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context, got %v", err)
	}

	if repo.calls != 0 || tokens.calls != 0 {
		t.Fatal("canceled login must not continue")
	}
}
