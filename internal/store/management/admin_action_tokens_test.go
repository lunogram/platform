package management

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminActionTokensStore(t *testing.T) {
	t.Parallel()
	db, raw := newContainerStoreWithDB(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Action Token Organization")
	require.NoError(t, err)

	newAdmin := func(t *testing.T, email string) uuid.UUID {
		t.Helper()
		adminID, err := db.CreateAdmin(ctx, Admin{OrganizationID: orgID, Email: email, Role: "owner"})
		require.NoError(t, err)
		return adminID
	}

	t.Run("redeems a token once", func(t *testing.T) {
		adminID := newAdmin(t, "redeem@example.com")

		plaintext, hash, err := NewActionToken()
		require.NoError(t, err)
		require.NoError(t, db.CreateAdminActionToken(ctx, adminID, ActionTokenPasswordReset, hash, PasswordResetTTL))

		redeemed, err := db.ConsumeAdminActionToken(ctx, ActionTokenPasswordReset, HashActionToken(plaintext))
		require.NoError(t, err)
		assert.Equal(t, adminID, redeemed)

		_, err = db.ConsumeAdminActionToken(ctx, ActionTokenPasswordReset, HashActionToken(plaintext))
		assert.ErrorIs(t, err, sql.ErrNoRows, "a consumed token must not be redeemable again")
	})

	// The guarded UPDATE is what makes single use real: two browsers following
	// the same link at the same moment must not both come away with a reset.
	t.Run("only one of many concurrent redemptions succeeds", func(t *testing.T) {
		adminID := newAdmin(t, "concurrent@example.com")

		plaintext, hash, err := NewActionToken()
		require.NoError(t, err)
		require.NoError(t, db.CreateAdminActionToken(ctx, adminID, ActionTokenPasswordReset, hash, PasswordResetTTL))

		const attempts = 8
		var (
			start     = make(chan struct{})
			wg        sync.WaitGroup
			mu        sync.Mutex
			succeeded int
		)

		wg.Add(attempts)
		for range attempts {
			go func() {
				defer wg.Done()
				<-start
				if _, err := db.ConsumeAdminActionToken(ctx, ActionTokenPasswordReset, HashActionToken(plaintext)); err == nil {
					mu.Lock()
					succeeded++
					mu.Unlock()
				}
			}()
		}
		close(start)
		wg.Wait()

		assert.Equal(t, 1, succeeded, "exactly one redemption may win")
	})

	t.Run("refuses a token issued for another purpose", func(t *testing.T) {
		adminID := newAdmin(t, "purpose@example.com")

		plaintext, hash, err := NewActionToken()
		require.NoError(t, err)
		require.NoError(t, db.CreateAdminActionToken(ctx, adminID, ActionTokenEmailVerification, hash, EmailVerificationTTL))

		_, err = db.ConsumeAdminActionToken(ctx, ActionTokenPasswordReset, HashActionToken(plaintext))
		assert.ErrorIs(t, err, sql.ErrNoRows, "a verification link must not be redeemable as a password reset")

		redeemed, err := db.ConsumeAdminActionToken(ctx, ActionTokenEmailVerification, HashActionToken(plaintext))
		require.NoError(t, err)
		assert.Equal(t, adminID, redeemed)
	})

	t.Run("refuses an expired token", func(t *testing.T) {
		adminID := newAdmin(t, "expired@example.com")

		plaintext, hash, err := NewActionToken()
		require.NoError(t, err)
		require.NoError(t, db.CreateAdminActionToken(ctx, adminID, ActionTokenPasswordReset, hash, -time.Minute))

		_, err = db.ConsumeAdminActionToken(ctx, ActionTokenPasswordReset, HashActionToken(plaintext))
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("refuses an unknown token", func(t *testing.T) {
		plaintext, _, err := NewActionToken()
		require.NoError(t, err)

		_, err = db.ConsumeAdminActionToken(ctx, ActionTokenPasswordReset, HashActionToken(plaintext))
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("invalidates the outstanding tokens of one purpose", func(t *testing.T) {
		adminID := newAdmin(t, "invalidate@example.com")

		reset, resetHash, err := NewActionToken()
		require.NoError(t, err)
		require.NoError(t, db.CreateAdminActionToken(ctx, adminID, ActionTokenPasswordReset, resetHash, PasswordResetTTL))

		verify, verifyHash, err := NewActionToken()
		require.NoError(t, err)
		require.NoError(t, db.CreateAdminActionToken(ctx, adminID, ActionTokenEmailVerification, verifyHash, EmailVerificationTTL))

		require.NoError(t, db.InvalidateAdminActionTokens(ctx, adminID, ActionTokenPasswordReset))

		_, err = db.ConsumeAdminActionToken(ctx, ActionTokenPasswordReset, HashActionToken(reset))
		assert.ErrorIs(t, err, sql.ErrNoRows, "an in-flight reset link must die with a password change")

		_, err = db.ConsumeAdminActionToken(ctx, ActionTokenEmailVerification, HashActionToken(verify))
		assert.NoError(t, err, "a token of another purpose must survive")
	})

	// The plaintext is what lands in an inbox; only its hash may reach the
	// table, or a database dump is a pile of working reset links.
	t.Run("stores only the hash", func(t *testing.T) {
		adminID := newAdmin(t, "hashonly@example.com")

		plaintext, hash, err := NewActionToken()
		require.NoError(t, err)
		require.NoError(t, db.CreateAdminActionToken(ctx, adminID, ActionTokenPasswordReset, hash, PasswordResetTTL))

		var stored []byte
		require.NoError(t, raw.GetContext(ctx, &stored,
			`SELECT token_hash FROM admin_action_tokens WHERE admin_id = $1`, adminID))

		assert.NotEqual(t, plaintext, string(stored))
		assert.Equal(t, HashActionToken(plaintext), stored)
	})

	t.Run("purges tokens that can no longer be redeemed", func(t *testing.T) {
		adminID := newAdmin(t, "purge@example.com")

		_, hash, err := NewActionToken()
		require.NoError(t, err)
		require.NoError(t, db.CreateAdminActionToken(ctx, adminID, ActionTokenPasswordReset, hash, -48*time.Hour))

		purged, err := db.PurgeExpiredAdminActionTokens(ctx, time.Hour)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, purged, int64(1))
	})
}

func TestNewActionTokenIsUnpredictable(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{}, 128)
	for range 128 {
		plaintext, hash, err := NewActionToken()
		require.NoError(t, err)
		require.NotEmpty(t, plaintext)
		require.Equal(t, HashActionToken(plaintext), hash)

		_, duplicate := seen[plaintext]
		require.False(t, duplicate, "generated the same token twice")
		seen[plaintext] = struct{}{}
	}
}
