package access

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProvisionMembershipOrdering covers the sequencing the doc comment on
// [ProvisionMembership] exists to protect: the database work is one transaction,
// and the OpenFGA tuples are written strictly AFTER it commits.
//
// OpenFGA is not part of the Postgres transaction, so getting this backwards
// would grant access for a membership that then rolled back.
func TestProvisionMembershipOrdering(t *testing.T) {
	t.Parallel()

	mgmtDB, _, _ := teststore.RunPostgreSQL(t)
	mgmt := management.NewState(mgmtDB)
	engine := rbac.NewTestEngine(t)
	ctx := context.Background()

	orgID, err := mgmt.CreateOrganization(ctx, "Membership Org")
	require.NoError(t, err)

	t.Run("the tuple is written only once the membership is committed", func(t *testing.T) {
		var committedDuringResolve bool

		adminID, err := ProvisionMembership(ctx, mgmtDB, engine,
			func(ctx context.Context, tx *management.State) (uuid.UUID, Membership, error) {
				adminID, err := tx.CreateAdmin(ctx, management.Admin{
					OrganizationID: orgID, Email: "ordering@example.com", Role: "owner",
				})
				if err != nil {
					return uuid.Nil, Membership{}, err
				}

				// Read through a SEPARATE connection: inside the transaction the
				// admin is invisible to everyone else, which is what makes the
				// post-commit tuple write meaningful.
				_, err = mgmt.GetAdmin(ctx, adminID)
				committedDuringResolve = err == nil

				return adminID, Membership{OrganizationID: orgID, Role: rbac.OrganizationOwner}, nil
			})
		require.NoError(t, err)

		assert.False(t, committedDuringResolve, "the admin must not be visible outside the transaction")

		member, err := mgmt.IsMember(ctx, orgID, adminID)
		require.NoError(t, err)
		assert.True(t, member)

		allowed, err := engine.Check(ctx, "user:"+adminID.String(), rbac.OrganizationOwner, rbac.OrganizationScope(orgID))
		require.NoError(t, err)
		assert.True(t, allowed, "the owner tuple must exist once the membership is committed")
	})

	t.Run("a failure inside the transaction leaves neither a membership nor a tuple", func(t *testing.T) {
		orphan := uuid.New()
		boom := errors.New("resolve failed")

		_, err := ProvisionMembership(ctx, mgmtDB, engine,
			func(ctx context.Context, tx *management.State) (uuid.UUID, Membership, error) {
				if _, err := tx.CreateAdmin(ctx, management.Admin{
					OrganizationID: orgID, Email: "rolled-back@example.com", Role: "owner",
				}); err != nil {
					return uuid.Nil, Membership{}, err
				}
				return orphan, Membership{OrganizationID: orgID, Role: rbac.OrganizationOwner}, boom
			})
		require.ErrorIs(t, err, boom)

		_, err = mgmt.GetAdminByEmail(ctx, "rolled-back@example.com")
		require.Error(t, err, "the admin insert must have rolled back with the transaction")

		allowed, err := engine.Check(ctx, "user:"+orphan.String(), rbac.OrganizationOwner, rbac.OrganizationScope(orgID))
		require.NoError(t, err)
		assert.False(t, allowed, "no tuple may be written for a membership that was never persisted")
	})

}
