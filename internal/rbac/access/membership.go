package access

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
)

// Membership is the organization an admin is granted access to and the role
// they hold in it.
type Membership struct {
	OrganizationID uuid.UUID
	Role           string
}

// ProvisionMembership grants an admin membership of an organization atomically:
// it records (or revives) the organization_members row inside a database
// transaction and, only after that transaction commits, reconciles the RBAC
// tuples in OpenFGA.
//
// Splitting the two phases this way is deliberate. OpenFGA is not part of the
// Postgres transaction, so writing tuples first could grant access for a
// membership that later rolls back. Doing the DB work in a transaction and the
// tuple write post-commit means a partial failure can never strand a membership
// without its role tuples or, worse, leave RBAC access for a membership that was
// never persisted. The invite-accept flow follows the same ordering; this helper
// exists so the admin-management paths reuse it instead of duplicating the
// fragile sequencing.
//
// resolve runs inside the transaction and returns the admin id that the
// membership is granted to, together with the organization and role to grant it
// in. It is where any prerequisite DB work (e.g. inserting a brand-new admin
// record, or creating the organization that admin will own) happens, sharing the
// transaction so it commits or rolls back together with the membership. The
// membership upsert runs after resolve returns successfully.
//
// The membership target comes back from resolve rather than being passed in
// because the login path only learns which organization a new admin joins while
// inside the transaction that creates it. Callers that already know both simply
// close over them.
//
// This is the ONLY membership provisioning path. The login flow used to carry
// its own copy that was neither transactional nor post-commit ordered, and that
// copy was the one every first login ran; it has been deleted in favour of this
// one.
func ProvisionMembership(
	ctx context.Context,
	db *sqlx.DB,
	engine *rbac.Engine,
	resolve func(ctx context.Context, tx *management.State) (uuid.UUID, Membership, error),
) (uuid.UUID, error) {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	store := management.NewState(tx)

	adminID, membership, err := resolve(ctx, store)
	if err != nil {
		return uuid.Nil, err
	}

	if err := store.AddMember(ctx, membership.OrganizationID, adminID, membership.Role); err != nil {
		return uuid.Nil, err
	}

	if err := tx.Commit(); err != nil {
		return uuid.Nil, err
	}

	// Tuples are reconciled only after the membership is durably committed; see
	// the doc comment for why the ordering matters.
	//
	// Both halves are idempotent, because both halves of a membership can already
	// exist: AddMember upserts the row and [SyncOrganizationRole] reconciles the
	// tuples to the committed role. Granting a membership somebody already holds
	// therefore succeeds instead of failing on the tuple write with the database
	// half already committed, and a role change drops the tuple for the role it
	// replaces instead of leaving the admin holding both.
	if engine != nil {
		if err := SyncOrganizationRole(ctx, engine, adminID, membership.OrganizationID, membership.Role); err != nil {
			return uuid.Nil, err
		}
	}

	return adminID, nil
}

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
