package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

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

	generated, err := generateVerificationToken(time.Now().UTC())

	if err != nil {
		return uuid.Nil, err
	}

	eventID := uuid.New()

	if err := s.outbox.Enqueue(ctx, eventID, userID, generated.Raw, generated.Hash, generated.ExpiresAt); err != nil {
		return uuid.Nil, fmt.Errorf("queue verification email: %w", err)
	}

	return eventID, nil
}
