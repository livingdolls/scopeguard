package domain

import (
	"errors"
	"unicode/utf8"
)

const (
	minPasswordLength = 15
	maxPasswordBytes  = 1024
)

var ErrInvalidPassword = errors.New("password does not meet requirements")

func ValidatePassword(password string) error {
	if !utf8.ValidString(password) {
		return ErrInvalidPassword
	}

	if utf8.RuneCountInString(password) < minPasswordLength {
		return ErrInvalidPassword
	}

	if len(password) > maxPasswordBytes {
		return ErrInvalidPassword
	}

	return nil
}
