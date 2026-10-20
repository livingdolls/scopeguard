package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type verificationRepositoryStub struct {
	eventID      uuid.UUID
	ownerID      uuid.UUID
	rawToken     string
	storedHash   []byte
	expiresAt    time.Time
	storeCalls   int
	consumeCalls int
	err          error
}

func (f *verificationRepositoryStub) Enqueue(
	ctx context.Context,
	eventID uuid.UUID,
	userID uuid.UUID,
	rawToken string,
	tokenHash []byte,
	expiresAt time.Time,
) error {
	f.storeCalls++

	if f.err != nil {
		return f.err
	}

	f.eventID = eventID
	f.ownerID = userID
	f.rawToken = rawToken
	f.storedHash = bytes.Clone(tokenHash)
	f.expiresAt = expiresAt

	return nil
}

func (f *verificationRepositoryStub) ConsumeAndActivate(
	ctx context.Context,
	tokenHash []byte,
) error {
	f.consumeCalls++

	if f.err != nil {
		return f.err
	}

	if !bytes.Equal(tokenHash, f.storedHash) {
		return ErrInvalidVerificationToken
	}

	return nil
}

func TestIssueEmailVerification(t *testing.T) {
	repo := &verificationRepositoryStub{}
	service := NewIssueEmailVerificationService(repo)

	ownerID := uuid.New()

	eventID, err := service.Execute(
		context.Background(),
		ownerID,
	)
	if err != nil {
		t.Fatal(err)
	}

	raw, err := base64.RawURLEncoding.DecodeString(
		repo.rawToken,
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(raw) != 32 {
		t.Fatal("expected 32 random token bytes")
	}

	digest := sha256.Sum256(raw)

	if !bytes.Equal(repo.storedHash, digest[:]) {
		t.Fatal("repository received incorrect token hash")
	}

	if repo.eventID != eventID || eventID == uuid.Nil {
		t.Fatal("unexpected outbox event ID")
	}

	if repo.ownerID != ownerID {
		t.Fatal("incorrect token owner")
	}

	if repo.storeCalls != 1 {
		t.Fatal("expected one repository call")
	}

	if repo.expiresAt.Before(time.Now().Add(23 * time.Hour)) {
		t.Fatal("verification token expires too early")
	}
}

func TestIssueEmailVerificationInvalidOwner(t *testing.T) {
	repo := &verificationRepositoryStub{}
	service := NewIssueEmailVerificationService(repo)

	eventID, err := service.Execute(context.Background(), uuid.Nil)

	if !errors.Is(err, ErrVerificationNotAllowed) {
		t.Fatalf("unexpected error: %v", err)
	}

	if eventID != uuid.Nil {
		t.Fatal("invalid owner must not receive an event ID")
	}

	if repo.storeCalls != 0 {
		t.Fatal("repository must not be called")
	}
}

func TestIssueEmailVerificationEnqueueFailure(t *testing.T) {
	repo := &verificationRepositoryStub{
		err: errors.New("outbox unavailable"),
	}
	service := NewIssueEmailVerificationService(repo)

	eventID, err := service.Execute(
		context.Background(),
		uuid.New(),
	)

	if !errors.Is(err, repo.err) {
		t.Fatalf("expected outbox error, got %v", err)
	}

	if eventID != uuid.Nil {
		t.Fatal("failed issuance must not return an event ID")
	}

	if repo.storeCalls != 1 {
		t.Fatal("expected one outbox call")
	}
}

func TestVerifyEmail(t *testing.T) {
	repo := &verificationRepositoryStub{}

	issuer := NewIssueEmailVerificationService(repo)
	verifier := NewVerifyEmailService(repo)

	_, err := issuer.Execute(
		context.Background(),
		uuid.New(),
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := verifier.Execute(
		context.Background(),
		repo.rawToken,
	); err != nil {
		t.Fatal(err)
	}

	if repo.consumeCalls != 1 {
		t.Fatal("expected one verification call")
	}
}

func TestVerifyEmailRejectsMalformedToken(t *testing.T) {
	repo := &verificationRepositoryStub{}
	verifier := NewVerifyEmailService(repo)

	invalidTokens := []string{
		"",
		"not-a-valid-token",
		"abc",
	}

	for _, token := range invalidTokens {
		err := verifier.Execute(
			context.Background(),
			token,
		)

		if !errors.Is(err, ErrInvalidVerificationToken) {
			t.Fatalf("expected invalid token, got %v", err)
		}
	}

	if repo.consumeCalls != 0 {
		t.Fatal("malformed token must not reach repository")
	}
}
