package application

import "errors"

var (
	ErrOrganizationSlugAlreadyExists = errors.New("organization slug already exists")
)
