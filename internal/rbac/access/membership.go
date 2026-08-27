package access

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rbac"
)

// OrganizationRoles returns the organization-level roles an admin can hold,
// highest first. An admin holds exactly one of them per organization:
// organization_members stores a single role per (organization_id, admin_id),
// and [SyncOrganizationRole] keeps the RBAC tuples matching that shape.
func OrganizationRoles() []string {
	return []string{rbac.OrganizationOwner, rbac.OrganizationAdmin, rbac.OrganizationMember}
}

// SyncOrganizationRole makes role the admin's only organization-level role in
// organizationID: it revokes the tuples for every other organization role and
// grants the tuple for role. Re-granting a role the admin already holds is a
// no-op rather than an error.
//
// Reconciling rather than only writing is what makes a role CHANGE take effect.
// The membership upsert updates the role column in place, so writing the new
// role tuple alone would leave the previous one behind and an admin demoted
// from owner to member would keep owner privileges in OpenFGA.
//
// Revocation runs before the grant on purpose. The two writes are separate
// OpenFGA requests, so a failure between them leaves the admin with fewer
// privileges than the committed membership grants (recoverable by re-running
// this, which is idempotent) rather than more than it grants (not
// self-correcting). Re-granting an unchanged role touches no tuple the admin
// still needs, so the common path has no window at all.
//
// The role is validated up front so an unknown value cannot revoke the admin's
// real role and then fail on the grant.
//
// This talks to OpenFGA, which is not part of any database transaction. Callers
// must invoke it only AFTER the transaction that persists the membership has
// committed, so that a rollback can never leave RBAC access for a membership
// that was never stored.
func SyncOrganizationRole(ctx context.Context, engine *rbac.Engine, adminID, organizationID uuid.UUID, role string) error {
	roles := OrganizationRoles()
	if !slices.Contains(roles, role) {
		return fmt.Errorf("access: %q is not an organization role", role)
	}

	var superseded []rbac.Tuple
	for _, other := range roles {
		if other == role {
			continue
		}
		superseded = append(superseded, OrganizationRoleTuples(adminID, organizationID, other)...)
	}

	if err := engine.DeleteTuplesIfPresent(ctx, superseded); err != nil {
		return fmt.Errorf("access: failed to revoke superseded organization roles for admin %s in organization %s: %w", adminID, organizationID, err)
	}

	if err := engine.WriteTuplesIfAbsent(ctx, OrganizationRoleTuples(adminID, organizationID, role)); err != nil {
		return fmt.Errorf("access: failed to grant organization role %q to admin %s in organization %s: %w", role, adminID, organizationID, err)
	}

	return nil
}
