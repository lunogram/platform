package management

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminSessionsStore(t *testing.T) {
	t.Parallel()
	db := NewContainerStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Session Organization")
	require.NoError(t, err)

	adminID, err := db.CreateAdmin(ctx, Admin{OrganizationID: orgID, Email: "session@example.com", Role: "owner"})
	require.NoError(t, err)

	now := time.Now()

	t.Run("creates and resolves a session", func(t *testing.T) {
		created, err := db.CreateAdminSession(ctx, AdminSession{
			AdminID:           adminID,
			ExpiresAt:         now.Add(8 * time.Hour),
			AbsoluteExpiresAt: now.Add(168 * time.Hour),
			Refreshable:       true,
			UserAgent:         ptr.To("console/1.0"),
			IP:                ptr.To("203.0.113.7"),
		})
		require.NoError(t, err)
		assert.True(t, created.Active(now))

		got, err := db.GetAdminSession(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, adminID, got.AdminID)
		require.NotNil(t, got.IP)
		assert.Equal(t, "203.0.113.7", *got.IP)
	})

	t.Run("revoking ends the session", func(t *testing.T) {
		created, err := db.CreateAdminSession(ctx, AdminSession{
			AdminID:           adminID,
			ExpiresAt:         now.Add(8 * time.Hour),
			AbsoluteExpiresAt: now.Add(168 * time.Hour),
			Refreshable:       true,
		})
		require.NoError(t, err)

		require.NoError(t, db.RevokeAdminSession(ctx, created.ID))

		got, err := db.GetAdminSession(ctx, created.ID)
		require.NoError(t, err)
		assert.False(t, got.Active(time.Now()), "a revoked session must not authenticate a request")

		_, err = db.RefreshAdminSession(ctx, created.ID, time.Now().Add(8*time.Hour))
		require.Error(t, err, "a revoked session must not be refreshable back to life")
	})

	t.Run("refresh cannot outlive the absolute expiry", func(t *testing.T) {
		created, err := db.CreateAdminSession(ctx, AdminSession{
			AdminID:           adminID,
			ExpiresAt:         time.Now().Add(time.Minute),
			AbsoluteExpiresAt: time.Now().Add(10 * time.Minute),
			Refreshable:       true,
		})
		require.NoError(t, err)

		refreshed, err := db.RefreshAdminSession(ctx, created.ID, time.Now().Add(72*time.Hour))
		require.NoError(t, err)
		assert.WithinDuration(t, created.AbsoluteExpiresAt, refreshed.ExpiresAt, time.Second,
			"the idle window must be clamped to the absolute expiry")
	})

	t.Run("deleting the admin ends every session they hold", func(t *testing.T) {
		doomed, err := db.CreateAdmin(ctx, Admin{OrganizationID: orgID, Email: "doomed@example.com", Role: "owner"})
		require.NoError(t, err)

		created, err := db.CreateAdminSession(ctx, AdminSession{
			AdminID:           doomed,
			ExpiresAt:         time.Now().Add(8 * time.Hour),
			AbsoluteExpiresAt: time.Now().Add(168 * time.Hour),
			Refreshable:       true,
		})
		require.NoError(t, err)

		require.NoError(t, db.DeleteAdmin(ctx, doomed))

		got, err := db.GetAdminSession(ctx, created.ID)
		require.NoError(t, err)
		assert.False(t, got.Active(time.Now()), "a deleted admin must not keep a working console")
	})

	t.Run("unlinking an identity ends the sessions it established", func(t *testing.T) {
		owner, err := db.CreateAdmin(ctx, Admin{OrganizationID: orgID, Email: "unlink@example.com", Role: "owner"})
		require.NoError(t, err)

		identityID, err := db.CreateAdminIdentity(ctx, AdminIdentity{
			AdminID: owner, Provider: IdentityProviderClerk,
			Issuer: "https://idp.example", Subject: "user_unlink",
		})
		require.NoError(t, err)

		created, err := db.CreateAdminSession(ctx, AdminSession{
			AdminID:           owner,
			AdminIdentityID:   &identityID,
			ExpiresAt:         time.Now().Add(8 * time.Hour),
			AbsoluteExpiresAt: time.Now().Add(168 * time.Hour),
			Refreshable:       true,
		})
		require.NoError(t, err)

		require.NoError(t, db.DeleteAdminIdentity(ctx, identityID))

		got, err := db.GetAdminSession(ctx, created.ID)
		require.NoError(t, err)
		assert.False(t, got.Active(time.Now()))
	})
}

// TestAdminSessionImpersonationConstraints proves the impersonation invariants
// are enforced by the database, not merely by the code that writes the row. A
// future code path that forgets to clamp the lifetime, to record the
// impersonator, or to mark the session non-refreshable must be unable to persist
// an over-privileged session at all.
func TestAdminSessionImpersonationConstraints(t *testing.T) {
	t.Parallel()
	db := NewContainerStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Impersonation Organization")
	require.NoError(t, err)

	adminID, err := db.CreateAdmin(ctx, Admin{OrganizationID: orgID, Email: "target@example.com", Role: "owner"})
	require.NoError(t, err)

	now := time.Now()
	upstream := now.Add(30 * time.Minute)

	valid := func() AdminSession {
		return AdminSession{
			AdminID:             adminID,
			Impersonated:        true,
			ImpersonatorSubject: ptr.To("user_support_engineer"),
			UpstreamExpiresAt:   &upstream,
			ExpiresAt:           upstream,
			AbsoluteExpiresAt:   upstream,
			Refreshable:         false,
		}
	}

	t.Run("a correctly clamped impersonated session is accepted", func(t *testing.T) {
		_, err := db.CreateAdminSession(ctx, valid())
		require.NoError(t, err)
	})

	tests := map[string]struct {
		mutate func(*AdminSession)
		reason string
	}{
		"refreshable": {
			mutate: func(s *AdminSession) { s.Refreshable = true },
			reason: "an impersonated session must never be extendable",
		},
		"no impersonator subject": {
			mutate: func(s *AdminSession) { s.ImpersonatorSubject = nil },
			reason: "an impersonated session must always record who is impersonating",
		},
		"no upstream expiry": {
			mutate: func(s *AdminSession) { s.UpstreamExpiresAt = nil },
			reason: "an impersonated session must be bounded by the upstream session",
		},
		"outliving the upstream session": {
			mutate: func(s *AdminSession) { s.AbsoluteExpiresAt = upstream.Add(time.Hour) },
			reason: "an impersonated session must not outlive the session that authorised it",
		},
		"idle window past the absolute expiry": {
			mutate: func(s *AdminSession) { s.ExpiresAt = s.AbsoluteExpiresAt.Add(time.Minute) },
			reason: "the idle window must never exceed the absolute lifetime",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			session := valid()
			tc.mutate(&session)
			_, err := db.CreateAdminSession(ctx, session)
			require.Error(t, err, tc.reason)
		})
	}

	t.Run("impersonator fields require the impersonated flag", func(t *testing.T) {
		impersonator, err := db.CreateAdmin(ctx, Admin{OrganizationID: orgID, Email: "staff@example.com", Role: "owner"})
		require.NoError(t, err)

		_, err = db.CreateAdminSession(ctx, AdminSession{
			AdminID:             adminID,
			Impersonated:        false,
			ImpersonatorAdminID: &impersonator,
			ImpersonatorSubject: ptr.To("user_support_engineer"),
			ExpiresAt:           now.Add(time.Hour),
			AbsoluteExpiresAt:   now.Add(time.Hour),
			Refreshable:         true,
		})
		require.Error(t, err, "a session that records an impersonator must be marked impersonated")
	})

	t.Run("an admin cannot impersonate themselves", func(t *testing.T) {
		_, err := db.CreateAdminSession(ctx, AdminSession{
			AdminID:             adminID,
			Impersonated:        true,
			ImpersonatorAdminID: &adminID,
			ImpersonatorSubject: ptr.To("user_self"),
			UpstreamExpiresAt:   &upstream,
			ExpiresAt:           upstream,
			AbsoluteExpiresAt:   upstream,
			Refreshable:         false,
		})
		require.Error(t, err)
	})

	t.Run("the impersonated admin must exist", func(t *testing.T) {
		session := valid()
		session.AdminID = uuid.New()
		_, err := db.CreateAdminSession(ctx, session)
		require.Error(t, err)
	})
}
