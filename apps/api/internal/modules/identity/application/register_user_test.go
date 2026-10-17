package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/domain"
)

type fakeRegistrationRepository struct {
	user  *domain.User
	hash  string
	err   error
	calls int
}

func (f *fakeRegistrationRepository) CreateWithPassword(
	ctx context.Context,
	user *domain.User,
	hash string,
) error {
	f.calls++

	if f.err != nil {
		return f.err
	}

	f.user = user
	f.hash = hash
	return nil
}

type fakePasswordHasher struct {
	received string
	err      error
	calls    int
}

func (f *fakePasswordHasher) Hash(password string) (string, error) {
	f.calls++
	f.received = password

	if f.err != nil {
		return "", f.err
	}

	return "test-encoded-hash", nil
}

func (f *fakePasswordHasher) Verify(
	password, encodedHash string,
) (bool, error) {
	return false, nil
}

func TestRegisterUserSuccess(t *testing.T) {
	repo := &fakeRegistrationRepository{}
	hasher := &fakePasswordHasher{}

	service := NewRegisterUserService(repo, hasher)

	result, err := service.Execute(
		context.Background(),
		RegisterUserInput{
			Email:    "  DEVELOPER@EXAMPLE.COM  ",
			Password: "correct horse battery staple",
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if result.ID == uuid.Nil {
		t.Fatal("expected valid UUID")
	}

	if result.Email != "developer@example.com" {
		t.Fatalf("unexpected email: %s", result.Email)
	}

	if repo.calls != 1 || hasher.calls != 1 {
		t.Fatal("expected one hash and one persistence call")
	}

	if repo.user == nil || repo.user.ID() != result.ID {
		t.Fatal("persisted user differs from output")
	}

	if repo.hash != "test-encoded-hash" {
		t.Fatal("repository did not receive encoded hash")
	}

	if hasher.received != "correct horse battery staple" {
		t.Fatal("hasher received incorrect password")
	}
}

func TestRegisterUserInvalidInput(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
		wantErr  error
	}{
		{
			name:     "invalid email",
			email:    "invalid-email",
			password: "correct horse battery staple",
			wantErr:  domain.ErrInvalidEmail,
		},
		{
			name:     "invalid password",
			email:    "user@example.com",
			password: "short",
			wantErr:  domain.ErrInvalidPassword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRegistrationRepository{}
			hasher := &fakePasswordHasher{}
			service := NewRegisterUserService(repo, hasher)

			_, err := service.Execute(
				context.Background(),
				RegisterUserInput{
					Email:    tt.email,
					Password: tt.password,
				},
			)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}

			if hasher.calls != 0 || repo.calls != 0 {
				t.Fatal("invalid input must not be processed")
			}
		})
	}
}

func TestRegisterUserPersistenceError(t *testing.T) {
	repo := &fakeRegistrationRepository{
		err: ErrEmailAlreadyExists,
	}
	hasher := &fakePasswordHasher{}
	service := NewRegisterUserService(repo, hasher)

	_, err := service.Execute(
		context.Background(),
		RegisterUserInput{
			Email:    "user@example.com",
			Password: "correct horse battery staple",
		},
	)

	if !errors.Is(err, ErrEmailAlreadyExists) {
		t.Fatalf("expected duplicate email error, got %v", err)
	}
}

func TestRegisterUserHashError(t *testing.T) {
	repo := &fakeRegistrationRepository{}
	hashErr := errors.New("hash failure")
	hasher := &fakePasswordHasher{err: hashErr}
	service := NewRegisterUserService(repo, hasher)

	_, err := service.Execute(
		context.Background(),
		RegisterUserInput{
			Email:    "user@example.com",
			Password: "correct horse battery staple",
		},
	)

	if !errors.Is(err, hashErr) {
		t.Fatalf("expected hash error, got %v", err)
	}

	if repo.calls != 0 {
		t.Fatal("must not persist user if hashing fails")
	}
}

func TestRegisterUserCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	repo := &fakeRegistrationRepository{}
	hasher := &fakePasswordHasher{}
	service := NewRegisterUserService(repo, hasher)

	_, err := service.Execute(
		ctx,
		RegisterUserInput{
			Email:    "user@example.com",
			Password: "correct horse battery staple",
		},
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled context, got %v", err)
	}

	if hasher.calls != 0 || repo.calls != 0 {
		t.Fatal("canceled request must not be processed")
	}
}
