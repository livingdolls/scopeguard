package application

import (
	"context"

	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/domain"
)

type PasswordHasher interface {
	Hash(plainPassword string) (string, error)
	Verify(plainPassword string, encodeHash string) (bool, error)
}

type RegistrationRepository interface {
	CreateWithPassword(ctx context.Context, user *domain.User, passwordHash string) error
}
