package application

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type EmailOutboxJob struct {
	ID       uuid.UUID
	LeaseID  uuid.UUID
	Attempts int
	Payload  EncryptedEmailPayload
}

type EmailOutboxJobRepository interface {
	Claim(ctx context.Context, lease time.Duration, maxAttempts int) (EmailOutboxJob, bool, error)
	CanSend(ctx context.Context, job EmailOutboxJob) (bool, error)
	MarkSent(ctx context.Context, job EmailOutboxJob) error
	MarkDead(ctx context.Context, job EmailOutboxJob) error
	MarkRetry(ctx context.Context, job EmailOutboxJob, availableAt time.Time) error
}

type VerificationEmail struct {
	To    string `json:"to"`
	Token string `json:"token"`
}

type EmailPayloadOpener interface {
	Open(eventID uuid.UUID, payload EncryptedEmailPayload) (VerificationEmail, error)
}

type EmailSender interface {
	SendVerification(ctx context.Context, to string, token string, deliveryID uuid.UUID) error
}
