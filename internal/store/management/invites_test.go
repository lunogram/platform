package management

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// inviteFixture creates an organization, a project and an inviter admin and
// returns the ids needed to exercise the invites store.
func inviteFixture(t *testing.T, db *State) (projectID, inviterID uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Invite Org")
	require.NoError(t, err)

	projectID, err = db.CreateProject(ctx, Project{
		OrganizationID: &orgID,
		Name:           "Invite Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	inviterID, err = db.CreateAdmin(ctx, Admin{
		OrganizationID: orgID,
		Email:          "inviter-" + uuid.NewString() + "@example.com",
		Role:           "owner",
	})
	require.NoError(t, err)

	return projectID, inviterID
}

func TestInvitesStore(t *testing.T) {
	t.Parallel()
	db, rawDB := newContainerStoreWithDB(t)
	ctx := context.Background()

	projectID, inviterID := inviteFixture(t, db)

	t.Run("creates invite with role and expiry", func(t *testing.T) {
		invite, err := db.CreateProjectInvite(ctx, projectID, inviterID, "new@example.com", nil, oapi.CreateProjectInviteRoleEditor, time.Hour)
		require.NoError(t, err)
		assert.Equal(t, "editor", invite.Role)
		assert.Equal(t, "new@example.com", invite.InviteeEmail)
		assert.Nil(t, invite.AcceptedAt)
		assert.Nil(t, invite.RevokedAt)
		// expires_at must be ~1h out, proving the seconds-based interval works.
		assert.WithinDuration(t, time.Now().Add(time.Hour), invite.ExpiresAt, 2*time.Minute)
	})

	t.Run("re-invite upserts the single pending row (ON CONFLICT)", func(t *testing.T) {
		first, err := db.CreateProjectInvite(ctx, projectID, inviterID, "dup@example.com", nil, oapi.CreateProjectInviteRoleSupport, time.Hour)
		require.NoError(t, err)

		second, err := db.CreateProjectInvite(ctx, projectID, inviterID, "dup@example.com", nil, oapi.CreateProjectInviteRoleAdmin, 2*time.Hour)
		require.NoError(t, err)

		// Same row id, new role/expiry.
		assert.Equal(t, first.ID, second.ID, "re-invite should upsert the same pending row")
		assert.Equal(t, "admin", second.Role)

		// Only one pending invite should exist for this email.
		invites, err := db.ListInvitesForEmail(ctx, "dup@example.com")
		require.NoError(t, err)
		assert.Len(t, invites, 1)
	})

	t.Run("accept marks accepted and is idempotent", func(t *testing.T) {
		invite, err := db.CreateProjectInvite(ctx, projectID, inviterID, "accept@example.com", nil, oapi.CreateProjectInviteRoleEditor, time.Hour)
		require.NoError(t, err)

		accepted, err := db.AcceptProjectInvite(ctx, invite.ID)
		require.NoError(t, err)
		require.NotNil(t, accepted.AcceptedAt)

		// A second accept matches no rows (guard excludes already-accepted).
		_, err = db.AcceptProjectInvite(ctx, invite.ID)
		assert.True(t, errors.Is(err, sql.ErrNoRows), "double accept should be a no-op (sql.ErrNoRows)")
	})

	t.Run("expired invite cannot be accepted", func(t *testing.T) {
		// Negative TTL would be rejected at the controller; here we drive the
		// store directly with a tiny TTL and wait for it to lapse to assert the
		// guarded UPDATE refuses an expired invite.
		invite, err := db.CreateProjectInvite(ctx, projectID, inviterID, "expired@example.com", nil, oapi.CreateProjectInviteRoleSupport, time.Second)
		require.NoError(t, err)

		// Force expiry deterministically.
		_, err = rawDB.ExecContext(ctx, `UPDATE project_invites SET expires_at = NOW() - INTERVAL '1 minute' WHERE id = $1`, invite.ID)
		require.NoError(t, err)

		_, err = db.AcceptProjectInvite(ctx, invite.ID)
		assert.True(t, errors.Is(err, sql.ErrNoRows), "expired invite must not be acceptable")
	})

	t.Run("revoked invite cannot be accepted and is excluded from my-invites", func(t *testing.T) {
		invite, err := db.CreateProjectInvite(ctx, projectID, inviterID, "revoked@example.com", nil, oapi.CreateProjectInviteRoleSupport, time.Hour)
		require.NoError(t, err)

		revoked, err := db.RevokeProjectInvite(ctx, invite.ID)
		require.NoError(t, err)
		require.NotNil(t, revoked.RevokedAt)

		_, err = db.AcceptProjectInvite(ctx, invite.ID)
		assert.True(t, errors.Is(err, sql.ErrNoRows), "revoked invite must not be acceptable")

		invites, err := db.ListInvitesForEmail(ctx, "revoked@example.com")
		require.NoError(t, err)
		assert.Empty(t, invites, "revoked invites are not surfaced as pending")
	})

	t.Run("my-invites matches email case-insensitively", func(t *testing.T) {
		_, err := db.CreateProjectInvite(ctx, projectID, inviterID, "MixedCase@Example.com", nil, oapi.CreateProjectInviteRoleSupport, time.Hour)
		require.NoError(t, err)

		invites, err := db.ListInvitesForEmail(ctx, "mixedcase@example.com")
		require.NoError(t, err)
		require.Len(t, invites, 1)
		assert.Equal(t, projectID, invites[0].ProjectID)
	})

	t.Run("rejects role outside the canonical set via CHECK", func(t *testing.T) {
		// "owner" is an organization role, not a project role; the DB CHECK must
		// reject it (and the spec no longer offers it).
		_, err := db.CreateProjectInvite(ctx, projectID, inviterID, "owner@example.com", nil, oapi.CreateProjectInviteRole("owner"), time.Hour)
		require.Error(t, err)
	})

	t.Run("list project invites filters and counts", func(t *testing.T) {
		// Fresh project to isolate counts.
		pID, iID := inviteFixture(t, db)
		_, err := db.CreateProjectInvite(ctx, pID, iID, "a@example.com", nil, oapi.CreateProjectInviteRoleEditor, time.Hour)
		require.NoError(t, err)
		_, err = db.CreateProjectInvite(ctx, pID, iID, "b@example.com", nil, oapi.CreateProjectInviteRoleSupport, time.Hour)
		require.NoError(t, err)

		invites, total, err := db.ListProjectInvites(ctx, pID, store.Pagination{Limit: 10, Offset: 0}, "", nil, nil, nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, invites, 2)

		// Search filter is parameterized and case-insensitive on the email.
		invites, total, err = db.ListProjectInvites(ctx, pID, store.Pagination{Limit: 10, Offset: 0}, "a@", nil, nil, nil, nil, nil)
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		require.Len(t, invites, 1)
		assert.Equal(t, "a@example.com", invites[0].InviteeEmail)
	})
}

func TestGetAdminByEmailCaseInsensitive(t *testing.T) {
	t.Parallel()
	db := NewContainerStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Case Org")
	require.NoError(t, err)

	_, err = db.CreateAdmin(ctx, Admin{
		OrganizationID: orgID,
		Email:          "CaseUser@Example.com",
		Role:           "owner",
	})
	require.NoError(t, err)

	// Lookup with a differently-cased email must still find the admin so the
	// cross-org authorization guard cannot be bypassed by email casing.
	admin, err := db.GetAdminByEmail(ctx, "caseuser@example.com")
	require.NoError(t, err)
	assert.Equal(t, "CaseUser@Example.com", admin.Email)
}
