package domain

import (
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidUserID   = errors.New("invalid user id")
	ErrInvalidEmail    = errors.New("invalid email")
	ErrInvalidUserTime = errors.New("invalid user timestamp")
)

type User struct {
	id        uuid.UUID
	email     string
	createdAt time.Time
	updatedAt time.Time
}

func NewUser(id uuid.UUID, email string, now time.Time) (*User, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidUserID
	}

	email, err := NormalizeEmail(email)
	if err != nil {
		return nil, err
	}

	if now.IsZero() {
		return nil, ErrInvalidUserTime
	}

	now = now.UTC()

	return &User{
		id:        id,
		email:     email,
		createdAt: now,
		updatedAt: now,
	}, nil
}

func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if len(email) == 0 || len(email) > 320 {
		return "", ErrInvalidEmail
	}

	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || address.Name != "" {
		return "", ErrInvalidEmail
	}

	return email, nil
}

func (u *User) ID() uuid.UUID {
	return u.id
}

func (u *User) Email() string {
	return u.email
}

func (u *User) CreatedAt() time.Time {
	return u.createdAt
}

func (u *User) UpdatedAt() time.Time {
	return u.updatedAt
}
