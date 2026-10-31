package application

import "errors"

var (
	ErrRoleSlugAlreadyExists         = errors.New("role slug already exists in organizations")
	ErrRoleOrganizationNotFound      = errors.New("role organization not found")
	ErrPermissionAssignmentForbidden = errors.New("permission assignment forbidden")
	ErrTargetRoleNotFound            = errors.New("target role not found")
	ErrSystemRoleImmutable           = errors.New("system role permissions cannot be modified")
	ErrPermissionNotFound            = errors.New("permission not found")
)
