package domain

const (
	PermissionOrganizationRead   = "organization:read"
	PermissionOrganizationUpdate = "organization:update"
	PermissionOrganizationDelete = "organization:delete"

	PermissionMemberRead       = "member:read"
	PermissionMemberInvite     = "member:invite"
	PermissionMemberRemove     = "member:remove"
	PermissionMemberRoleAssign = "member:role_assign"

	PermissionRoleRead   = "role:read"
	PermissionRoleCreate = "role:create"
	PermissionRoleUpdate = "role:update"
	PermissionRoleDelete = "role:delete"

	PermissionPermissionRead = "permission:read"
)

func BuiltInPermissionCodes() []string {
	return []string{
		PermissionOrganizationRead,
		PermissionOrganizationUpdate,
		PermissionOrganizationDelete,
		PermissionMemberRead,
		PermissionMemberInvite,
		PermissionMemberRemove,
		PermissionMemberRoleAssign,
		PermissionRoleRead,
		PermissionRoleCreate,
		PermissionRoleUpdate,
		PermissionRoleDelete,
		PermissionPermissionRead,
	}
}
