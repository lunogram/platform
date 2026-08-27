package verifiers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/password"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const verifierPassword = "an entirely ordinary passphrase"

type passwordVerifierEnv struct {
	verifier *Password
	mgmt     *management.State
	orgID    uuid.UUID
}

func newPasswordVerifierEnv(t *testing.T) *passwordVerifierEnv {
	t.Helper()

	mgmtDB, _, _ := teststore.RunPostgreSQL(t)
	mgmt := management.NewState(mgmtDB)

	orgID, err := mgmt.CreateOrganization(t.Context(), "Verifier Organization")
	require.NoError(t, err)

	return &passwordVerifierEnv{
		verifier: NewPassword(mgmt, zaptest.NewLogger(t)),
		mgmt:     mgmt,
		orgID:    orgID,
	}
}

// admin creates an account with a local password identity.
func (e *passwordVerifierEnv) admin(t *testing.T, email, plain string, verified bool) (uuid.UUID, uuid.UUID) {
	t.Helper()

	adminID, err := e.mgmt.CreateAdmin(t.Context(), management.Admin{
		OrganizationID: e.orgID, Email: email, Role: "owner",
	})
	require.NoError(t, err)

	hash, err := password.Hash(plain)
	require.NoError(t, err)

	identityID, err := e.mgmt.CreateAdminIdentity(t.Context(), management.AdminIdentity{
		AdminID:       adminID,
		Provider:      management.IdentityProviderPassword,
		Issuer:        management.PasswordIssuer,
		Subject:       adminID.String(),
		Email:         ptr.To(email),
		EmailVerified: verified,
		SecretHash:    ptr.To(hash),
	})
	require.NoError(t, err)

	return adminID, identityID
}

func passwordRequest(t *testing.T, body any) *http.Request {
	t.Helper()

	encoded, err := json.Marshal(body)
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodPost, "/api/auth/login/password/callback", bytes.NewReader(encoded))
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestPasswordDriver(t *testing.T) {
	t.Parallel()
	assert.Equal(t, PasswordDriver, NewPassword(nil, zaptest.NewLogger(t)).Driver())
}

func TestPasswordVerify(t *testing.T) {
	t.Parallel()
	env := newPasswordVerifierEnv(t)
	ctx := context.Background()

	adminID, _ := env.admin(t, "verified@example.test", verifierPassword, true)
	env.admin(t, "unverified@example.test", verifierPassword, false)

	t.Run("proves a correct credential", func(t *testing.T) {
		identity, err := env.verifier.Verify(ctx, passwordRequest(t, map[string]string{
			"email": "verified@example.test", "password": verifierPassword,
		}))
		require.NoError(t, err)

		assert.Equal(t, management.PasswordIssuer, identity.Issuer)
		assert.Equal(t, adminID.String(), identity.Subject)
		assert.Equal(t, management.IdentityProviderPassword, identity.Provider)
		assert.Equal(t, "verified@example.test", identity.Email)
		assert.True(t, identity.EmailVerified)

		// A verifier proves a credential and stops: nothing about the account
		// changes and no session is opened.
		assert.Nil(t, identity.Actor)
	})

	t.Run("the address is matched case-insensitively", func(t *testing.T) {
		identity, err := env.verifier.Verify(ctx, passwordRequest(t, map[string]string{
			"email": "  VERIFIED@Example.Test ", "password": verifierPassword,
		}))
		require.NoError(t, err)
		assert.Equal(t, adminID.String(), identity.Subject)
	})

	// The stored flag is the only source: a login must never be able to assert
	// that the address behind it is verified, or the exchange's email-linking
	// gate is worth nothing.
	t.Run("an unverified account signs in and stays unverified", func(t *testing.T) {
		identity, err := env.verifier.Verify(ctx, passwordRequest(t, map[string]string{
			"email": "unverified@example.test", "password": verifierPassword,
		}))
		require.NoError(t, err)
		assert.False(t, identity.EmailVerified)
	})

	t.Run("rejects a wrong password", func(t *testing.T) {
		_, err := env.verifier.Verify(ctx, passwordRequest(t, map[string]string{
			"email": "verified@example.test", "password": "not the right passphrase",
		}))
		assert.ErrorIs(t, err, ErrInvalidCredentials)
	})

	// Every failure has to look the same. A distinguishable "no such account"
	// is an account list waiting to be walked.
	t.Run("an unknown address fails exactly like a wrong password", func(t *testing.T) {
		_, err := env.verifier.Verify(ctx, passwordRequest(t, map[string]string{
			"email": "nobody@example.test", "password": verifierPassword,
		}))
		assert.ErrorIs(t, err, ErrInvalidCredentials)
	})

	t.Run("an account with no password fails the same way", func(t *testing.T) {
		adminID, err := env.mgmt.CreateAdmin(ctx, management.Admin{
			OrganizationID: env.orgID, Email: "federated@example.test", Role: "owner",
		})
		require.NoError(t, err)
		_, err = env.mgmt.CreateAdminIdentity(ctx, management.AdminIdentity{
			AdminID: adminID, Provider: management.IdentityProviderClerk,
			Issuer: "https://idp.test", Subject: "user_federated",
		})
		require.NoError(t, err)

		_, err = env.verifier.Verify(ctx, passwordRequest(t, map[string]string{
			"email": "federated@example.test", "password": verifierPassword,
		}))
		assert.ErrorIs(t, err, ErrInvalidCredentials)
	})

	t.Run("an empty credential is a bad request, not a failed login", func(t *testing.T) {
		for _, body := range []map[string]string{
			{"email": "", "password": verifierPassword},
			{"email": "verified@example.test", "password": ""},
		} {
			_, err := env.verifier.Verify(ctx, passwordRequest(t, body))
			assert.ErrorIs(t, err, ErrMissingCredentials)
		}
	})
}

// Raising the cost parameters must eventually reach the oldest hashes, which are
// exactly the ones a leak would crack first.
func TestPasswordVerifyRehashesOutdatedParameters(t *testing.T) {
	t.Parallel()
	env := newPasswordVerifierEnv(t)
	ctx := context.Background()

	adminID, identityID := env.admin(t, "outdated@example.test", verifierPassword, true)

	weaker := password.DefaultParams
	weaker.Memory /= 4
	weaker.Time = 1
	legacy, err := password.HashWith(verifierPassword, weaker)
	require.NoError(t, err)
	require.NoError(t, env.mgmt.SetAdminIdentitySecret(ctx, identityID, legacy))

	_, err = env.verifier.Verify(ctx, passwordRequest(t, map[string]string{
		"email": "outdated@example.test", "password": verifierPassword,
	}))
	require.NoError(t, err)

	identity, err := env.mgmt.GetPasswordIdentity(ctx, adminID)
	require.NoError(t, err)
	require.NotNil(t, identity.SecretHash)
	assert.NotEqual(t, legacy, *identity.SecretHash)

	_, outdated, err := password.Verify(*identity.SecretHash, verifierPassword)
	require.NoError(t, err)
	assert.False(t, outdated)
}

// Rehashing a guess would be worse than pointless, so a failed attempt must
// leave the stored hash exactly as it was.
func TestPasswordVerifyDoesNotRehashOnFailure(t *testing.T) {
	t.Parallel()
	env := newPasswordVerifierEnv(t)
	ctx := context.Background()

	adminID, identityID := env.admin(t, "untouched@example.test", verifierPassword, true)

	weaker := password.DefaultParams
	weaker.Time = 1
	legacy, err := password.HashWith(verifierPassword, weaker)
	require.NoError(t, err)
	require.NoError(t, env.mgmt.SetAdminIdentitySecret(ctx, identityID, legacy))

	_, err = env.verifier.Verify(ctx, passwordRequest(t, map[string]string{
		"email": "untouched@example.test", "password": "not the right passphrase",
	}))
	require.ErrorIs(t, err, ErrInvalidCredentials)

	identity, err := env.mgmt.GetPasswordIdentity(ctx, adminID)
	require.NoError(t, err)
	assert.Equal(t, legacy, *identity.SecretHash)
}

// A hash the database holds but this build cannot parse is a data problem, and
// there is no safe way to answer it other than as a failed login.
func TestPasswordVerifyRejectsAnUnreadableHash(t *testing.T) {
	t.Parallel()
	env := newPasswordVerifierEnv(t)
	ctx := context.Background()

	_, identityID := env.admin(t, "corrupt@example.test", verifierPassword, true)
	require.NoError(t, env.mgmt.SetAdminIdentitySecret(ctx, identityID, "not a hash at all"))

	_, err := env.verifier.Verify(ctx, passwordRequest(t, map[string]string{
		"email": "corrupt@example.test", "password": verifierPassword,
	}))
	assert.ErrorIs(t, err, ErrInvalidCredentials)
}
