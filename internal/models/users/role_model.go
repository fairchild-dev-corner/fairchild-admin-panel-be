package models

import (
	"slices"

	cc "fairchild-admin-panel-be/internal/constants"
)

// Roles lists the panel roles from highest to lowest. Mirrored by
// PANEL_ROLES in the admin panel (src/constants/roles.ts).
var Roles = []string{cc.ROLE_SUPER_ADMIN, cc.ROLE_ADMIN, cc.ROLE_MANAGER, cc.ROLE_SUPERVISOR, cc.ROLE_STAFF}

// AdminRoles may open Users and Capabilities and have every capability.
var AdminRoles = []string{cc.ROLE_SUPER_ADMIN, cc.ROLE_ADMIN}

// Rank orders roles: staff 1 up to super admin 5, 0 for anything else.
func Rank(role string) int {
	i := slices.Index(Roles, role)
	if i < 0 {
		return 0
	}
	return len(Roles) - i
}

// IsAdmin reports whether role has every capability.
func IsAdmin(role string) bool {
	return slices.Contains(AdminRoles, role)
}

// HasAssignedCapabilities reports whether role's capabilities are set per
// account (managers, supervisors and staff).
func HasAssignedCapabilities(role string) bool {
	return Rank(role) > 0 && !IsAdmin(role)
}

// CanAssign reports whether a user with actorRole may give role to an
// account: a role below their own, or any role for a super admin.
func CanAssign(actorRole, role string) bool {
	if Rank(role) == 0 || !IsAdmin(actorRole) {
		return false
	}
	return actorRole == cc.ROLE_SUPER_ADMIN || Rank(role) < Rank(actorRole)
}

// CanManage reports whether the actor may change the target account (role,
// status, capabilities): never their own, and only below their own role
// unless they're a super admin.
func CanManage(actorID int64, actorRole string, targetID int64, targetRole string) bool {
	return actorID != targetID && CanAssign(actorRole, targetRole)
}
