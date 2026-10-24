package domain

import (
	"errors"
	"regexp"
	"strings"
)

var (
	ErrInvalidPermissionResource = errors.New("invalid permission resource")
	ErrInvalidPermissionAction   = errors.New("invalid permission action")
	ErrInvalidPermissionCode     = errors.New("invalid permission code")
)

var permissionSegmentPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

type Permission struct {
	resource string
	action   string
}

func NewPermission(resource string, action string) (Permission, error) {
	if !permissionSegmentPattern.MatchString(resource) {
		return Permission{}, ErrInvalidPermissionResource
	}

	if !permissionSegmentPattern.MatchString(action) {
		return Permission{}, ErrInvalidPermissionAction
	}

	return Permission{
		resource: resource,
		action:   action,
	}, nil
}

func ParsePermission(code string) (Permission, error) {
	resource, action, found := strings.Cut(code, ":")

	if !found {
		return Permission{}, ErrInvalidPermissionCode
	}

	permission, err := NewPermission(resource, action)
	if err != nil {
		return Permission{}, ErrInvalidPermissionCode
	}

	return permission, nil
}

func (p Permission) Resource() string {
	return p.resource
}

func (p Permission) Action() string {
	return p.action
}

func (p Permission) Code() string {
	return p.resource + ":" + p.action
}
