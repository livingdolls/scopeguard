package application

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/domain"
)

type PasswordHasher interface {
	Hash(plainPassword string) (string, error)
	Verify(plainPassword string, encodeHash string) (bool, error)
}

type RegistrationRepository interface {
	CreateWithPassword(ctx context.Context, user *domain.User, passwordHash string) error
}

type LoginCredential struct {
	UserID       uuid.UUID
	PasswordHash string
	Status       domain.AccountStatus
}

type LoginRepository interface {
	FindByEmail(ctx context.Context, email string) (LoginCredential, error)
}

type AccessTokenIssuer interface {
	Issue(userID uuid.UUID) (string, time.Time, error)
}

type AccessTokenVerifier interface {
	Verify(token string) (uuid.UUID, error)
}

type UserAccessChecker interface {
	CanAuthenticate(ctx context.Context, userID uuid.UUID) (bool, error)
}

type EmailVerificationRepository interface {
	ConsumeAndActivate(ctx context.Context, tokenHash []byte) error
}
type EmailVerificationOutbox interface {
	Enqueue(ctx context.Context, eventID uuid.UUID, userID uuid.UUID, rawToken string, tokenHash []byte, expiresAt time.Time) error
}

type EncryptedEmailPayload struct {
	KeyID string
	Data  []byte
}

type EmailPayloadSealer interface {
	Seal(eventID uuid.UUID, email string, token string) (EncryptedEmailPayload, error)
}
