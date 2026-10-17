package domain

import "errors"

var (
	ErrInvalidOrganizationID   = errors.New("invalid organization ID")
	ErrInvalidOrganizationName = errors.New("invalid organization name")
	ErrInvalidOrganizationSlug = errors.New("invalid organization slug")
	ErrInvalidOrganizationTime = errors.New("invalid organization time")
)
