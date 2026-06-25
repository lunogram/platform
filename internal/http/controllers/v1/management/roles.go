package v1

import "github.com/lunogram/platform/internal/rbac"

// effectiveProjectRole applies org→project inheritance to an admin's explicit
// project role. An organization owner or admin is a project admin by
// inheritance (see the OpenFGA "project" type, where project admin = ttu
// organization owner/admin), so they always resolve to project admin regardless
// of any explicit project_admins row. Everyone else gets their explicit role,
// which is "" when they have none — a value that ranks below every real role
// and therefore grants no authority and gates no UI.
//
// This is the single source of truth for the role shown to and enforced for an
// admin in a project at the application layer; the SQL stays free of role
// business logic and backend authorization is always enforced via OpenFGA.
func effectiveProjectRole(orgRole, explicitProjectRole string) string {
	if orgRole == rbac.OrganizationOwner || orgRole == rbac.OrganizationAdmin {
		return rbac.ProjectAdmin
	}
	return explicitProjectRole
}
