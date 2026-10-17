package password

import (
	"fmt"

	"github.com/alexedwards/argon2id"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/application"
	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/domain"
)

type Argon2idHasher struct {
	params argon2id.Params
}

var _ application.PasswordHasher = (*Argon2idHasher)(nil)

func NewArgon2idHasher() *Argon2idHasher {
	return &Argon2idHasher{
		params: argon2id.Params{
			Memory:      19 * 1024,
			Iterations:  2,
			Parallelism: 1,
			SaltLength:  16,
			KeyLength:   32,
		},
	}
}

func (a Argon2idHasher) Hash(plainPassword string) (string, error) {
	if err := domain.ValidatePassword(plainPassword); err != nil {
		return "", err
	}

	encodeHash, err := argon2id.CreateHash(plainPassword, &a.params)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	return encodeHash, nil
}

func (a Argon2idHasher) Verify(plainPassword string, encodeHash string) (bool, error) {
	match, err := argon2id.ComparePasswordAndHash(plainPassword, encodeHash)

	if err != nil {
		return false, fmt.Errorf("verify password: %w", err)
	}

	return match, nil
}
