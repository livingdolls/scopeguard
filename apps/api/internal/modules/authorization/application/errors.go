package application

import "errors"

var (
	ErrRoleSlugAlreadyExists    = errors.New("role slug already exists in organizations")
	ErrRoleOrganizationNotFound = errors.New("role organization not found")
)
