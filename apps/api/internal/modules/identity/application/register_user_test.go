package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/domain"
)

type fakeRegistrationRepository struct {
	record RegistrationRecord
	err    error
	calls  int
}

func (f *fakeRegistrationRepository) RegisterWithVerification(
	ctx context.Context,
	record RegistrationRecord,
) error {
	f.calls++

	if f.err != nil {
		return f.err
	}

	f.record = record
	return nil
}

type fakeRegistrationSealer struct {
	eventID uuid.UUID
	email   string
	token   string
	calls   int
	err     error
}

func (f *fakeRegistrationSealer) Seal(
	eventID uuid.UUID,
	email string,
	token string,
) (EncryptedEmailPayload, error) {
	f.calls++

	if f.err != nil {
		return EncryptedEmailPayload{}, f.err
	}

	f.eventID = eventID
	f.email = email
	f.token = token

	return EncryptedEmailPayload{
		KeyID: "test-key",
		Data:  []byte("fake-encrypted-payload"),
	}, nil
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
	sealer := &fakeRegistrationSealer{}

	service := NewRegisterUserService(repo, hasher, sealer)

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

	if repo.calls != 1 || hasher.calls != 1 || sealer.calls != 1 {
		t.Fatal("expected one hash, encryption, and persistence call")
	}

	if repo.record.User == nil {
		t.Fatal("user was not persisted")
	}

	if repo.record.User.ID() != result.ID {
		t.Fatal("registered user ID mismatch")
	}

	if repo.record.PasswordHash != "test-encoded-hash" {
		t.Fatal("repository received incorrect password hash")
	}

	if repo.record.VerificationID == uuid.Nil ||
		repo.record.OutboxID == uuid.Nil {
		t.Fatal("missing verification or outbox ID")
	}

	if sealer.eventID != repo.record.OutboxID {
		t.Fatal("encryption event ID mismatch")
	}

	if sealer.email != "developer@example.com" {
		t.Fatal("encryption received incorrect email")
	}

	raw, err := base64.RawURLEncoding.DecodeString(
		sealer.token,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(raw) != 32 {
		t.Fatal("expected 32 random token bytes")
	}

	digest := sha256.Sum256(raw)

	if !bytes.Equal(repo.record.TokenHash, digest[:]) {
		t.Fatal("repository received incorrect token hash")
	}

	if repo.record.EncryptedPayload.KeyID != "test-key" ||
		!bytes.Equal(repo.record.EncryptedPayload.Data, []byte("fake-encrypted-payload")) {
		t.Fatal("repository received incorrect encrypted payload")
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
			sealer := &fakeRegistrationSealer{}
			service := NewRegisterUserService(repo, hasher, sealer)

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

			if sealer.calls != 0 {
				t.Fatal("invalid input must not be encrypted")
			}
		})
	}
}

func TestRegisterUserPersistenceError(t *testing.T) {
	repo := &fakeRegistrationRepository{
		err: ErrEmailAlreadyExists,
	}
	hasher := &fakePasswordHasher{}
	sealer := &fakeRegistrationSealer{}
	service := NewRegisterUserService(repo, hasher, sealer)

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
	sealer := &fakeRegistrationSealer{}
	service := NewRegisterUserService(repo, hasher, sealer)

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

	if sealer.calls != 0 {
		t.Fatal("hashing failure must prevent encryption")
	}
}

func TestRegisterUserEncryptionError(t *testing.T) {
	repo := &fakeRegistrationRepository{}
	hasher := &fakePasswordHasher{}

	encryptionErr := errors.New("encryption unavailable")

	sealer := &fakeRegistrationSealer{
		err: encryptionErr,
	}

	service := NewRegisterUserService(repo, hasher, sealer)

	_, err := service.Execute(
		context.Background(),
		RegisterUserInput{
			Email:    "user@example.com",
			Password: "correct horse battery staple",
		},
	)

	if !errors.Is(err, encryptionErr) {
		t.Fatalf("expected encryption error, got %v", err)
	}

	if repo.calls != 0 {
		t.Fatal("encryption failure must prevent persistence")
	}
}

func TestRegisterUserCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	repo := &fakeRegistrationRepository{}
	hasher := &fakePasswordHasher{}
	sealer := &fakeRegistrationSealer{}
	service := NewRegisterUserService(repo, hasher, sealer)

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

	if sealer.calls != 0 {
		t.Fatal("canceled request must not be encrypted")
	}
}
