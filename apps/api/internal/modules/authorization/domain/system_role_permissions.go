package domain

// DefaultSystemRolePermissions defines the initial
// permissions granted when a system role is provisioned.
//
// Changes to this policy require a corresponding
// database migration for existing organizations.
func DefaultSystemRolePermissions(slug string) []string {
	switch slug {
	case RoleOwner:
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

	case RoleMember:
		return []string{
			PermissionOrganizationRead,
			PermissionMemberRead,
			PermissionRoleRead,
			PermissionPermissionRead,
		}

	default:
		return nil
	}
}
