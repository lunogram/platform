package auth

import (
	"context"
	"testing"

	"github.com/lunogram/platform/internal/password"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const seedPassword = "an entirely ordinary passphrase"

func newSeeder(t *testing.T, env *exchangeEnv) *Seeder {
	t.Helper()
	return NewSeeder(env.exchanger, env.mgmt, env.db, zaptest.NewLogger(t))
}

// A fresh deployment has to be reachable the moment it starts.
func TestSeedCreatesTheConfiguredAccount(t *testing.T) {
	t.Parallel()

	env := newExchangeEnv(t)
	ctx := context.Background()

	require.NoError(t, newSeeder(t, env).Seed(ctx, "Owner@Example.com", seedPassword))

	admin, err := env.mgmt.ResolveAdminByEmail(ctx, "owner@example.com")
	require.NoError(t, err)

	identity, err := env.mgmt.GetLocalIdentity(ctx, admin.ID)
	require.NoError(t, err)
	require.NotNil(t, identity.SecretHash)

	match, err := password.Verify(*identity.SecretHash, seedPassword)
	require.NoError(t, err)
	assert.True(t, match, "the configured password must be the one that was stored")

	assert.Equal(t, management.IdentityProviderBasic, identity.Provider)
	assert.Equal(t, admin.ID.String(), identity.Subject,
		"the subject is the admin id, so the credential does not follow an address change")
	assert.True(t, identity.EmailVerified,
		"the operator configured the address, so there is nobody else it could belong to")
}

// The upgrade path: a deployment that has been signing in with the configured
// pair already has the admin, and only the stored hash is new.
func TestSeedFillsAMissingSecretOnAnExistingAdmin(t *testing.T) {
	t.Parallel()

	env := newExchangeEnv(t)
	ctx := context.Background()

	orgID, err := env.mgmt.CreateOrganization(ctx, "Existing")
	require.NoError(t, err)
	adminID, err := env.mgmt.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID, Email: "owner@example.com", Role: "owner",
	})
	require.NoError(t, err)

	require.NoError(t, newSeeder(t, env).Seed(ctx, "owner@example.com", seedPassword))

	identity, err := env.mgmt.GetLocalIdentity(ctx, adminID)
	require.NoError(t, err)
	require.NotNil(t, identity.SecretHash)

	match, err := password.Verify(*identity.SecretHash, seedPassword)
	require.NoError(t, err)
	assert.True(t, match)

	admin, err := env.mgmt.ResolveAdminByEmail(ctx, "owner@example.com")
	require.NoError(t, err)
	assert.Equal(t, adminID, admin.ID, "the existing admin keeps its id, organization and permissions")
}

// A deployment restarts for reasons that have nothing to do with credentials.
// Re-applying the environment would silently undo a password changed in the
// console -- the change would appear to work and then revert.
func TestSeedNeverReplacesAStoredPassword(t *testing.T) {
	t.Parallel()

	env := newExchangeEnv(t)
	ctx := context.Background()
	seeder := newSeeder(t, env)

	require.NoError(t, seeder.Seed(ctx, "owner@example.com", seedPassword))

	admin, err := env.mgmt.ResolveAdminByEmail(ctx, "owner@example.com")
	require.NoError(t, err)
	identity, err := env.mgmt.GetLocalIdentity(ctx, admin.ID)
	require.NoError(t, err)

	chosen, err := password.Hash("the password they chose in the console")
	require.NoError(t, err)
	require.NoError(t, env.mgmt.SetAdminIdentitySecret(ctx, identity.ID, chosen))

	require.NoError(t, seeder.Seed(ctx, "owner@example.com", "a different configured password"))

	after, err := env.mgmt.GetLocalIdentity(ctx, admin.ID)
	require.NoError(t, err)

	match, err := password.Verify(*after.SecretHash, "the password they chose in the console")
	require.NoError(t, err)
	assert.True(t, match, "the stored password must survive a restart")
}

// Seeding twice is what every restart does.
func TestSeedIsIdempotent(t *testing.T) {
	t.Parallel()

	env := newExchangeEnv(t)
	ctx := context.Background()
	seeder := newSeeder(t, env)

	require.NoError(t, seeder.Seed(ctx, "owner@example.com", seedPassword))
	admin, err := env.mgmt.ResolveAdminByEmail(ctx, "owner@example.com")
	require.NoError(t, err)

	require.NoError(t, seeder.Seed(ctx, "owner@example.com", seedPassword))

	again, err := env.mgmt.ResolveAdminByEmail(ctx, "owner@example.com")
	require.NoError(t, err)
	assert.Equal(t, admin.ID, again.ID, "a restart must not create a second account")
}

// Configuring neither is how a deployment says it provisions admins some other
// way. Configuring one of the two is a mistake, and a silent one: nothing would
// be created and the operator would find out by not being able to sign in.
func TestSeedDoesNothingWithoutConfiguration(t *testing.T) {
	t.Parallel()

	env := newExchangeEnv(t)
	seeder := newSeeder(t, env)

	require.NoError(t, seeder.Seed(context.Background(), "", ""))
}

func TestSeedRefusesAHalfConfiguredPair(t *testing.T) {
	t.Parallel()

	env := newExchangeEnv(t)
	seeder := newSeeder(t, env)

	err := seeder.Seed(context.Background(), "owner@example.com", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AUTH_BASIC_PASSWORD")

	err = seeder.Seed(context.Background(), "", seedPassword)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AUTH_BASIC_EMAIL")
}

// A stored password is the admin's, not the environment's: once one exists the
// variable left behind in the environment must not quietly replace it.
func TestSeedIgnoresTheVariableOnceAPasswordIsStored(t *testing.T) {
	t.Parallel()

	env := newExchangeEnv(t)
	ctx := context.Background()
	seeder := newSeeder(t, env)

	require.NoError(t, seeder.Seed(ctx, "owner@example.com", seedPassword))
	require.NoError(t, seeder.Seed(ctx, "owner@example.com", "admin"))

	admin, err := env.mgmt.ResolveAdminByEmail(ctx, "owner@example.com")
	require.NoError(t, err)
	identity, err := env.mgmt.GetLocalIdentity(ctx, admin.ID)
	require.NoError(t, err)

	match, err := password.Verify(*identity.SecretHash, seedPassword)
	require.NoError(t, err)
	assert.True(t, match)
}
