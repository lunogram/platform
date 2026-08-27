package rbac

import "slices"

// projectRoleRank ranks the project-level roles for least-privilege comparisons.
// Project roles form a strict hierarchy: support < client < editor < admin.
// "owner" is an *organization*-level role, not a project role (the OpenFGA
// "project" type has no "owner" relation), so it is deliberately absent here —
// an unknown role ranks 0 and can never out-rank a real project role.
var projectRoleRank = map[string]int{
	ProjectSupport: 1,
	ProjectClient:  2,
	ProjectEditor:  3,
	ProjectAdmin:   4,
}

// IsProjectRoleHigher reports whether role1 out-ranks role2 in the project role
// hierarchy. Use it to enforce a least-privilege ceiling: a caller may only
// grant a role that does not out-rank their own.
func IsProjectRoleHigher(role1, role2 string) bool {
	return projectRoleRank[role1] > projectRoleRank[role2]
}

// IsKnownProjectRole reports whether role is one the hierarchy can rank. An
// unranked role would compare as 0 and slip under the least-privilege ceiling,
// so callers granting a role must reject anything this rejects.
func IsKnownProjectRole(role string) bool {
	_, ok := projectRoleRank[role]
	return ok
}

// projectRolesByRank lists the project roles from most to least privileged. Use
// it to find the highest role a subject holds: the relations are nested
// (admin implies editor implies client/support), so the first one that resolves
// is the subject's effective role.
func projectRolesByRank() []string {
	return []string{ProjectAdmin, ProjectEditor, ProjectClient, ProjectSupport}
}

// OrganizationRolesInheritingProjectAdmin returns the organization roles whose
// holders are project admins in every project of that organization, without
// needing an explicit grant. It mirrors the tuple-to-userset rewrite on the
// OpenFGA "project" type (project admin = organization owner/admin) and is the
// single source of truth for that inheritance outside the authorization model —
// both [EffectiveProjectRole] and the project-visibility SQL derive from it.
func OrganizationRolesInheritingProjectAdmin() []string {
	return []string{OrganizationOwner, OrganizationAdmin}
}

// EffectiveProjectRole applies org→project inheritance to an admin's explicit
// project role. An organization owner or admin is a project admin by
// inheritance, so they always resolve to project admin regardless of any
// explicit project_admins row. Everyone else gets their explicit role, which is
// "" when they have none — a value that ranks below every real role and
// therefore grants no authority and gates no UI.
//
// orgRole must be the caller's membership role in the organization that scopes
// the request, not the global role on the admins record: an admin can belong to
// several organizations, and their authority in one says nothing about another.
//
// This is the single source of truth for the role shown to and enforced for an
// admin in a project at the application layer; the SQL stays free of role
// business logic and backend authorization is always enforced via OpenFGA.
func EffectiveProjectRole(orgRole, explicitProjectRole string) string {
	if slices.Contains(OrganizationRolesInheritingProjectAdmin(), orgRole) {
		return ProjectAdmin
	}
	return explicitProjectRole
}
