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
	repository EmailVerificationRepository
}

func NewIssueEmailVerificationService(repository EmailVerificationRepository) *IssueEmailVerificationService {
	return &IssueEmailVerificationService{
		repository: repository,
	}
}

// Execute returns a secret token for trusted internal
// delivery only. never expose it through a public api
func (s *IssueEmailVerificationService) Execute(ctx context.Context, userID uuid.UUID) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if userID == uuid.Nil {
		return "", ErrVerificationNotAllowed
	}

	// generate a cryptography secure token
	raw := make([]byte, verificationTokenBytes)

	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("failed to generate verification token: %w", err)
	}

	// hash the token before persistence
	digest := sha256.Sum256(raw)

	expiresAt := time.Now().UTC().Add(verificationTokenTTL)

	// persist the hash
	if err := s.repository.StorePending(ctx, userID, digest[:], expiresAt); err != nil {
		return "", fmt.Errorf("failed to store verification token: %w", err)
	}

	// encode for delivery through a trusted channel
	token := base64.RawURLEncoding.EncodeToString(raw)

	return token, nil
}
