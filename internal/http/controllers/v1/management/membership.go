package v1

import (
	"context"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/rbac/access"
	"github.com/lunogram/platform/internal/store/management"
)

// provisionMembership grants an admin membership of an organization atomically:
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
// resolveAdmin runs inside the transaction and returns the admin id that the
// membership is granted to. It is where any prerequisite DB work (e.g. inserting
// a brand-new admin record) happens, sharing the transaction so it commits or
// rolls back together with the membership. The membership upsert runs after
// resolveAdmin returns successfully.
func provisionMembership(
	ctx context.Context,
	db *sqlx.DB,
	engine *rbac.Engine,
	organizationID uuid.UUID,
	role string,
	resolveAdmin func(ctx context.Context, tx *management.State) (uuid.UUID, error),
) (uuid.UUID, error) {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	store := management.NewState(tx)

	adminID, err := resolveAdmin(ctx, store)
	if err != nil {
		return uuid.Nil, err
	}

	if err := store.AddMember(ctx, organizationID, adminID, role); err != nil {
		return uuid.Nil, err
	}

	if err := tx.Commit(); err != nil {
		return uuid.Nil, err
	}

	// Tuples are written only after the membership is durably committed; see the
	// doc comment for why the ordering matters.
	if err := engine.WriteTuples(ctx, access.OrganizationRoleTuples(adminID, organizationID, role)); err != nil {
		return uuid.Nil, err
	}

	return adminID, nil
}
