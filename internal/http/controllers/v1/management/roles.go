package v1

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
)

// resolveProjectRole returns the admin's effective role in the project, folding
// their membership role in the organization that scopes the request together
// with any explicit project_admins row (see [rbac.EffectiveProjectRole]).
//
// The organization role MUST come from organization_members for the active
// organization, never from admins.role: that column is the admin's role in
// their *home* organization, so an owner of org A would otherwise resolve to
// project admin inside org B's projects.
//
// A missing membership or a missing project_admins row is not an error — both
// mean "no role here" and resolve to "", which ranks below every real role.
// Only a genuine query failure is returned, so a transient database error can
// never be mistaken for a role.
func resolveProjectRole(ctx context.Context, store *management.State, organizationID, projectID, adminID uuid.UUID) (string, error) {
	var orgRole string
	member, err := store.GetMember(ctx, organizationID, adminID)
	switch {
	case err == nil:
		orgRole = member.Role
	case errors.Is(err, sql.ErrNoRows):
	default:
		return "", err
	}

	explicitRole, err := store.GetProjectRole(ctx, projectID, adminID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	return rbac.EffectiveProjectRole(orgRole, explicitRole), nil
}
