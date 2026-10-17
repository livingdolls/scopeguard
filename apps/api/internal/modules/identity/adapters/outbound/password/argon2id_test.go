package password

import (
	"errors"
	"strings"
	"testing"

	"github.com/livingdolls/scopeguard/apps/api/internal/modules/identity/domain"
)

func TestArgon2idHashAndVerify(t *testing.T) {
	hasher := NewArgon2idHasher()
	plainPassword := "correct horse battery staple"

	hash, err := hasher.Hash(plainPassword)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatal("expected Argon2id encoded hash")
	}

	if hash == plainPassword {
		t.Fatal("plaintext password must not be stored")
	}

	matched, err := hasher.Verify(plainPassword, hash)
	if err != nil {
		t.Fatalf("verify password: %v", err)
	}

	if !matched {
		t.Fatal("correct password must match")
	}

	matched, err = hasher.Verify(
		"incorrect password",
		hash,
	)
	if err != nil {
		t.Fatalf("verify wrong password: %v", err)
	}

	if matched {
		t.Fatal("incorrect password must not match")
	}
}

func TestArgon2idGeneratesDifferentSalts(t *testing.T) {
	hasher := NewArgon2idHasher()
	password := "correct horse battery staple"

	first, err := hasher.Hash(password)
	if err != nil {
		t.Fatal(err)
	}

	second, err := hasher.Hash(password)
	if err != nil {
		t.Fatal(err)
	}

	if first == second {
		t.Fatal("hashes must use unique random salts")
	}
}

func TestArgon2idRejectsInvalidPassword(t *testing.T) {
	hasher := NewArgon2idHasher()

	_, err := hasher.Hash("short")

	if !errors.Is(err, domain.ErrInvalidPassword) {
		t.Fatalf("expected invalid password, got %v", err)
	}
}

func TestArgon2idRejectsMalformedHash(t *testing.T) {
	hasher := NewArgon2idHasher()

	matched, err := hasher.Verify(
		"correct horse battery staple",
		"not-an-argon2id-hash",
	)

	if err == nil {
		t.Fatal("expected malformed hash error")
	}

	if matched {
		t.Fatal("malformed hash must not match")
	}
}
