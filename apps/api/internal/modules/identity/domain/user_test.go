package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewUser(t *testing.T) {
	id := uuid.New()
	now := time.Now().UTC()

	user, err := NewUser(
		id,
		"  DEVELOPER@EXAMPLE.COM  ",
		now,
	)
	if err != nil {
		t.Fatal(err)
	}

	if user.ID() != id {
		t.Fatal("unexpected user ID")
	}

	if user.Email() != "developer@example.com" {
		t.Fatalf("unexpected email: %s", user.Email())
	}

	if !user.CreatedAt().Equal(now) {
		t.Fatal("unexpected createdAt")
	}
}

func TestNewUserInvalidEmail(t *testing.T) {
	invalidEmails := []string{
		"",
		"not-an-email",
		"name@",
		"Display Name <name@example.com>",
	}

	for _, email := range invalidEmails {
		_, err := NewUser(
			uuid.New(),
			email,
			time.Now(),
		)

		if !errors.Is(err, ErrInvalidEmail) {
			t.Fatalf("email %q: got %v", email, err)
		}
	}
}

func TestNewUserInvalidIdentity(t *testing.T) {
	_, err := NewUser(
		uuid.Nil,
		"user@example.com",
		time.Now(),
	)
	if !errors.Is(err, ErrInvalidUserID) {
		t.Fatalf("expected invalid user ID, got %v", err)
	}

	_, err = NewUser(
		uuid.New(),
		"user@example.com",
		time.Time{},
	)
	if !errors.Is(err, ErrInvalidUserTime) {
		t.Fatalf("expected invalid timestamp, got %v", err)
	}
}
