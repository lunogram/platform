package management

import (
	"context"
	"database/sql"
	"testing"

	"github.com/lunogram/platform/internal/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminIdentitiesStore(t *testing.T) {
	t.Parallel()
	db := NewContainerStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Identity Organization")
	require.NoError(t, err)

	t.Run("resolves an identity by its issuer and subject", func(t *testing.T) {
		adminID, err := db.CreateAdmin(ctx, Admin{OrganizationID: orgID, Email: "resolve@example.com", Role: "owner"})
		require.NoError(t, err)

		id, err := db.CreateAdminIdentity(ctx, AdminIdentity{
			AdminID:       adminID,
			Provider:      IdentityProviderClerk,
			Issuer:        "https://idp.example",
			Subject:       "user_resolve",
			Email:         ptr.To("resolve@example.com"),
			EmailVerified: true,
		})
		require.NoError(t, err)

		identity, err := db.GetAdminIdentity(ctx, "https://idp.example", "user_resolve")
		require.NoError(t, err)
		assert.Equal(t, id, identity.ID)
		assert.Equal(t, adminID, identity.AdminID)
		assert.True(t, identity.EmailVerified)
	})

	t.Run("the same subject under a different issuer is a different identity", func(t *testing.T) {
		// The identity key is (issuer, subject) precisely so two customers'
		// SSO connections can hand us the same subject without colliding.
		first, err := db.CreateAdmin(ctx, Admin{OrganizationID: orgID, Email: "tenant-a@example.com", Role: "owner"})
		require.NoError(t, err)
		second, err := db.CreateAdmin(ctx, Admin{OrganizationID: orgID, Email: "tenant-b@example.com", Role: "owner"})
		require.NoError(t, err)

		_, err = db.CreateAdminIdentity(ctx, AdminIdentity{
			AdminID: first, Provider: IdentityProviderSAML,
			Issuer: "https://tenant-a.example", Subject: "shared-subject",
		})
		require.NoError(t, err)

		_, err = db.CreateAdminIdentity(ctx, AdminIdentity{
			AdminID: second, Provider: IdentityProviderSAML,
			Issuer: "https://tenant-b.example", Subject: "shared-subject",
		})
		require.NoError(t, err, "same subject under a different issuer must be allowed")

		identity, err := db.GetAdminIdentity(ctx, "https://tenant-b.example", "shared-subject")
		require.NoError(t, err)
		assert.Equal(t, second, identity.AdminID)
	})

	t.Run("rejects a duplicate issuer and subject", func(t *testing.T) {
		owner, err := db.CreateAdmin(ctx, Admin{OrganizationID: orgID, Email: "dup-owner@example.com", Role: "owner"})
		require.NoError(t, err)
		other, err := db.CreateAdmin(ctx, Admin{OrganizationID: orgID, Email: "dup-other@example.com", Role: "owner"})
		require.NoError(t, err)

		_, err = db.CreateAdminIdentity(ctx, AdminIdentity{
			AdminID: owner, Provider: IdentityProviderClerk,
			Issuer: "https://idp.example", Subject: "user_duplicate",
		})
		require.NoError(t, err)

		_, err = db.CreateAdminIdentity(ctx, AdminIdentity{
			AdminID: other, Provider: IdentityProviderClerk,
			Issuer: "https://idp.example", Subject: "user_duplicate",
		})
		require.Error(t, err, "two admins must never claim the same upstream identity")
	})

	t.Run("a soft-deleted identity can be linked again", func(t *testing.T) {
		first, err := db.CreateAdmin(ctx, Admin{OrganizationID: orgID, Email: "relink-first@example.com", Role: "owner"})
		require.NoError(t, err)
		second, err := db.CreateAdmin(ctx, Admin{OrganizationID: orgID, Email: "relink-second@example.com", Role: "owner"})
		require.NoError(t, err)

		id, err := db.CreateAdminIdentity(ctx, AdminIdentity{
			AdminID: first, Provider: IdentityProviderClerk,
			Issuer: "https://idp.example", Subject: "user_relink",
		})
		require.NoError(t, err)
		require.NoError(t, db.DeleteAdminIdentity(ctx, id))

		_, err = db.GetAdminIdentity(ctx, "https://idp.example", "user_relink")
		require.ErrorIs(t, err, sql.ErrNoRows, "an unlinked identity must not resolve")

		relinked, err := db.CreateAdminIdentity(ctx, AdminIdentity{
			AdminID: second, Provider: IdentityProviderClerk,
			Issuer: "https://idp.example", Subject: "user_relink",
		})
		require.NoError(t, err, "the partial unique index must ignore unlinked rows")

		identity, err := db.GetAdminIdentity(ctx, "https://idp.example", "user_relink")
		require.NoError(t, err)
		assert.Equal(t, relinked, identity.ID)
		assert.Equal(t, second, identity.AdminID)
	})

	// Only a local identity may hold a secret. The reverse does not hold: a
	// local identity may be waiting for one -- an admin who holds an invite but
	// has not chosen a password yet, and the seeded account between the
	// migration that creates it and the boot that fills the hash in.
	t.Run("only a local identity may carry a secret", func(t *testing.T) {
		adminID, err := db.CreateAdmin(ctx, Admin{OrganizationID: orgID, Email: "secret@example.com", Role: "owner"})
		require.NoError(t, err)

		_, err = db.CreateAdminIdentity(ctx, AdminIdentity{
			AdminID: adminID, Provider: IdentityProviderClerk,
			Issuer: "https://idp.example", Subject: "user_with_secret",
			SecretHash: ptr.To("$argon2id$dummy"),
		})
		require.Error(t, err, "a federated identity must never carry a local secret")

		pending, err := db.CreateAdminIdentity(ctx, AdminIdentity{
			AdminID: adminID, Provider: IdentityProviderBasic,
			Issuer: LocalIssuer, Subject: adminID.String(),
		})
		require.NoError(t, err, "a local identity may exist before a password is chosen")

		require.NoError(t, db.SetAdminIdentitySecret(ctx, pending, "$argon2id$dummy"))

		stored, err := db.GetLocalIdentity(ctx, adminID)
		require.NoError(t, err)
		require.NotNil(t, stored.SecretHash)
		assert.Equal(t, "$argon2id$dummy", *stored.SecretHash)
	})

	// Re-hashing on login reads a hash, computes a stronger one and writes it
	// back. A password change committing in between must win, or maintenance
	// would restore the password the login proved.
	t.Run("a secret is only replaced when it is still the one that was read", func(t *testing.T) {
		adminID, err := db.CreateAdmin(ctx, Admin{OrganizationID: orgID, Email: "swap@example.com", Role: "owner"})
		require.NoError(t, err)

		id, err := db.CreateAdminIdentity(ctx, AdminIdentity{
			AdminID: adminID, Provider: IdentityProviderBasic,
			Issuer: LocalIssuer, Subject: adminID.String(),
			SecretHash: ptr.To("$argon2id$original"),
		})
		require.NoError(t, err)

		replaced, err := db.ReplaceAdminIdentitySecret(ctx, id, "$argon2id$original", "$argon2id$rehashed")
		require.NoError(t, err)
		assert.True(t, replaced)

		// Somebody changed the password after the login read the old hash.
		require.NoError(t, db.SetAdminIdentitySecret(ctx, id, "$argon2id$changed"))

		replaced, err = db.ReplaceAdminIdentitySecret(ctx, id, "$argon2id$rehashed", "$argon2id$stale")
		require.NoError(t, err)
		assert.False(t, replaced, "a re-hash of a superseded password must not land")

		stored, err := db.GetLocalIdentity(ctx, adminID)
		require.NoError(t, err)
		assert.Equal(t, "$argon2id$changed", *stored.SecretHash)
	})

	t.Run("rejects an unknown provider", func(t *testing.T) {
		adminID, err := db.CreateAdmin(ctx, Admin{OrganizationID: orgID, Email: "provider@example.com", Role: "owner"})
		require.NoError(t, err)

		_, err = db.CreateAdminIdentity(ctx, AdminIdentity{
			AdminID: adminID, Provider: "magic-link",
			Issuer: "https://idp.example", Subject: "user_unknown_provider",
		})
		require.Error(t, err)
	})

	t.Run("adoption only ever moves a row off the sentinel issuer", func(t *testing.T) {
		adminID, err := db.CreateAdmin(ctx, Admin{OrganizationID: orgID, Email: "adopt@example.com", Role: "owner"})
		require.NoError(t, err)

		id, err := db.CreateAdminIdentity(ctx, AdminIdentity{
			AdminID: adminID, Provider: IdentityProviderClerk,
			Issuer: LegacyExternalIDIssuer, Subject: "user_legacy",
		})
		require.NoError(t, err)

		require.NoError(t, db.AdoptLegacyIdentity(ctx, id, "https://idp.example", IdentityProviderClerk))

		identity, err := db.GetAdminIdentity(ctx, "https://idp.example", "user_legacy")
		require.NoError(t, err)
		assert.Equal(t, id, identity.ID, "adoption must rewrite the row in place, not replace it")

		// A second adoption attempt against a real issuer must be a no-op: the
		// statement may only ever move a row OFF the sentinel.
		require.NoError(t, db.AdoptLegacyIdentity(ctx, id, "https://attacker.example", IdentityProviderClerk))
		identity, err = db.GetAdminIdentity(ctx, "https://idp.example", "user_legacy")
		require.NoError(t, err)
		assert.Equal(t, "https://idp.example", identity.Issuer)
	})

	t.Run("records the last login", func(t *testing.T) {
		adminID, err := db.CreateAdmin(ctx, Admin{OrganizationID: orgID, Email: "touch@example.com", Role: "owner"})
		require.NoError(t, err)

		id, err := db.CreateAdminIdentity(ctx, AdminIdentity{
			AdminID: adminID, Provider: IdentityProviderClerk,
			Issuer: "https://idp.example", Subject: "user_touch",
		})
		require.NoError(t, err)

		require.NoError(t, db.TouchAdminIdentity(ctx, id, "renamed@example.com", true))

		identity, err := db.GetAdminIdentity(ctx, "https://idp.example", "user_touch")
		require.NoError(t, err)
		require.NotNil(t, identity.LastLoginAt)
		require.NotNil(t, identity.Email)
		assert.Equal(t, "renamed@example.com", *identity.Email)
		assert.True(t, identity.EmailVerified)

		identities, err := db.ListAdminIdentities(ctx, adminID)
		require.NoError(t, err)
		require.Len(t, identities, 1)
	})
}
