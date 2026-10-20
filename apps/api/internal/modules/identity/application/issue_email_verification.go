package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const verificationTokenTTL = 24 * time.Hour
const verificationTokenBytes = 32

type IssueEmailVerificationService struct {
	outbox EmailVerificationOutbox
}

func NewIssueEmailVerificationService(outbox EmailVerificationOutbox) *IssueEmailVerificationService {
	return &IssueEmailVerificationService{
		outbox: outbox,
	}
}

// Execute returns a secret token for trusted internal
// delivery only. never expose it through a public api
func (s *IssueEmailVerificationService) Execute(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	if err := ctx.Err(); err != nil {
		return uuid.Nil, err
	}

	if userID == uuid.Nil {
		return uuid.Nil, ErrVerificationNotAllowed
	}

	raw := make([]byte, verificationTokenBytes)

	if _, err := rand.Read(raw); err != nil {
		return uuid.Nil, fmt.Errorf("failed to generate verification token: %w", err)
	}

	rawToken := base64.RawURLEncoding.EncodeToString(raw)
	digest := sha256.Sum256(raw)

	eventID := uuid.New()
	expiresAt := time.Now().UTC().Add(verificationTokenTTL)

	if err := s.outbox.Enqueue(ctx, eventID, userID, rawToken, digest[:], expiresAt); err != nil {
		return uuid.Nil, fmt.Errorf("queue verification email: %w", err)
	}

	return eventID, nil
}
