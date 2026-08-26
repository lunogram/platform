package access

import (
	"context"

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
// transaction and, only after that transaction commits, writes the RBAC tuples
// to OpenFGA.
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

	// Tuples are written only after the membership is durably committed; see the
	// doc comment for why the ordering matters.
	if engine != nil {
		if err := engine.WriteTuples(ctx, OrganizationRoleTuples(adminID, membership.OrganizationID, membership.Role)); err != nil {
			return uuid.Nil, err
		}
	}

	return adminID, nil
}
