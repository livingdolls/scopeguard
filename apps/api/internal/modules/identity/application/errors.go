package application

import "errors"

var (
	ErrEmailAlreadyExists       = errors.New("email already exists")
	ErrInvalidCredentials       = errors.New("invalid credentials")
	ErrUserNotFound             = errors.New("user not found")
	ErrInvalidAccessToken       = errors.New("invalid access token")
	ErrInvalidVerificationToken = errors.New("invalid verification token")
	ErrVerificationNotAllowed   = errors.New("email verification is not allowed for this account")
)
