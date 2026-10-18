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

type UserExistenceChecker interface {
	Exists(ctx context.Context, userID uuid.UUID) (bool, error)
}
