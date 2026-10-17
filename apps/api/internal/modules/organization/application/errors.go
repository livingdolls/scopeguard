package application

import "errors"

var (
	ErrOrganizationSlugAlreadyExists = errors.New("organization slug already exists")
	ErrInvalidOwnerUserID            = errors.New("invalid owner user id")
	ErrOwnerUserNotFound             = errors.New("owner user not found")
)
