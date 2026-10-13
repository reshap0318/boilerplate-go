package dtos

// Role names.
var (
	RoleSuperAdmin = "Super Admin"
	RoleAdmin      = "Admin"
	RoleEditor     = "Editor"
	RoleViewer     = "Viewer"
)

// Permission names ({resource}.{action}).
var (
	PermissionUserIndex  = "user.index"
	PermissionUserCreate = "user.create"
	PermissionUserEdit   = "user.edit"
	PermissionUserDelete = "user.delete"

	PermissionRoleIndex   = "role.index"
	PermissionRoleIndexSu = "role.index-su"
	PermissionRoleCreate  = "role.create"
	PermissionRoleEdit    = "role.edit"
	PermissionRoleDelete  = "role.delete"

	PermissionIndex = "permission.index"
)

// RbacPermission defines a system permission.
type RbacPermission struct {
	Name        string
	Description string
}

// RbacRole defines a system role and its permissions.
type RbacRole struct {
	Name        string
	Description string
	Permissions []string
}

// RbacPermissions defines all system permissions.
var RbacPermissions = []RbacPermission{
	// User
	{Name: PermissionUserIndex, Description: "View users list"},
	{Name: PermissionUserCreate, Description: "Create new user"},
	{Name: PermissionUserEdit, Description: "Update user"},
	{Name: PermissionUserDelete, Description: "Delete user"},
	// Role
	{Name: PermissionRoleIndex, Description: "View roles list"},
	{Name: PermissionRoleIndexSu, Description: "View super admin role (ID 1)"},
	{Name: PermissionRoleCreate, Description: "Create new role"},
	{Name: PermissionRoleEdit, Description: "Update role"},
	{Name: PermissionRoleDelete, Description: "Delete role"},
	// Permission (read-only)
	{Name: PermissionIndex, Description: "View permissions list"},
}

// RbacRoles defines all system roles. Super Admin always receives every permission.
var RbacRoles = []RbacRole{
	{
		Name:        RoleSuperAdmin,
		Description: "Full access to all features",
	},
	{
		Name:        RoleAdmin,
		Description: "Manage users, roles, and permissions",
		Permissions: []string{
			PermissionUserIndex, PermissionUserCreate, PermissionUserEdit, PermissionUserDelete,
			PermissionRoleIndex, PermissionRoleCreate, PermissionRoleEdit, PermissionRoleDelete,
			PermissionIndex,
		},
	},
	{
		Name:        RoleEditor,
		Description: "Manage users and view data",
		Permissions: []string{
			PermissionUserIndex, PermissionUserCreate, PermissionUserEdit,
			PermissionRoleIndex,
			PermissionIndex,
		},
	},
	{
		Name:        RoleViewer,
		Description: "Read-only access",
		Permissions: []string{
			PermissionUserIndex,
			PermissionRoleIndex,
			PermissionIndex,
		},
	},
}
