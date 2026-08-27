package verifiers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const clerkIssuer = "https://clerk.test"

type clerkEnv struct {
	db       *sqlx.DB
	mgmt     *management.State
	verifier *Clerk
	key      *rsa.PrivateKey
}

func newClerkEnv(t *testing.T) *clerkEnv {
	t.Helper()

	mgmtDB, _, _ := teststore.RunPostgreSQL(t)
	mgmt := management.NewState(mgmtDB)
	logger := zaptest.NewLogger(t)

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	signer, err := auth.NewConsoleSigner(config.ConsoleAuth{
		SigningKey:  testECKey(t),
		Issuer:      "https://lunogram.test/console",
		Audience:    "lunogram-console",
		IdleTTL:     8 * time.Hour,
		AbsoluteTTL: 168 * time.Hour,
	})
	require.NoError(t, err)

	exchanger := auth.NewExchanger(mgmtDB, mgmt, rbac.NewTestEngine(t), signer, nil, logger, true)

	keyFunc := func(*jwt.Token) (any, error) { return &key.PublicKey, nil }
	verifier, err := NewClerk(
		config.ClerkAuth{SecretKey: "sk_test_xxx", Issuer: clerkIssuer},
		mgmt, logger, keyFunc, exchanger,
	)
	require.NoError(t, err)

	return &clerkEnv{db: mgmtDB, mgmt: mgmt, verifier: verifier, key: key}
}

func (e *clerkEnv) sign(t *testing.T, method jwt.SigningMethod, claims jwt.MapClaims) string {
	t.Helper()
	var key any = e.key
	if _, ok := method.(*jwt.SigningMethodHMAC); ok {
		key = []byte("shared-secret")
	}
	signed, err := jwt.NewWithClaims(method, claims).SignedString(key)
	require.NoError(t, err)
	return signed
}

func clerkRequest(token string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/api/auth/login/clerk/callback", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	return r
}

func TestClerkDriver(t *testing.T) {
	t.Parallel()
	verifier, err := NewClerk(config.ClerkAuth{SecretKey: "sk_test_xxx"}, nil, zaptest.NewLogger(t), nil, nil)
	require.NoError(t, err)
	require.Equal(t, "clerk", verifier.Driver())
}

func TestClerkVerify(t *testing.T) {
	t.Parallel()
	env := newClerkEnv(t)
	ctx := context.Background()

	baseClaims := func() jwt.MapClaims {
		return jwt.MapClaims{
			"iss":            clerkIssuer,
			"sub":            "user_verify",
			"exp":            time.Now().Add(time.Hour).Unix(),
			"email":          "verify@example.com",
			"email_verified": true,
		}
	}

	t.Run("proves the identity carried by the token", func(t *testing.T) {
		identity, err := env.verifier.Verify(ctx, clerkRequest(env.sign(t, jwt.SigningMethodRS256, baseClaims())))
		require.NoError(t, err)

		assert.Equal(t, clerkIssuer, identity.Issuer)
		assert.Equal(t, "user_verify", identity.Subject)
		assert.Equal(t, management.IdentityProviderClerk, identity.Provider)
		assert.Equal(t, "verify@example.com", identity.Email)
		assert.True(t, identity.EmailVerified)
		assert.Nil(t, identity.Actor)
		assert.False(t, identity.ExpiresAt.IsZero())
	})

	t.Run("HS256 is rejected", func(t *testing.T) {
		// Accepting a symmetric algorithm against an asymmetric keyfunc is the
		// shape of the classic algorithm-confusion attack. This verifier has no
		// reason to accept one at all.
		_, err := env.verifier.Verify(ctx, clerkRequest(env.sign(t, jwt.SigningMethodHS256, baseClaims())))
		require.ErrorIs(t, err, ErrInvalidToken)
	})

	t.Run("a token without an expiry is rejected", func(t *testing.T) {
		claims := baseClaims()
		delete(claims, "exp")
		_, err := env.verifier.Verify(ctx, clerkRequest(env.sign(t, jwt.SigningMethodRS256, claims)))
		require.ErrorIs(t, err, ErrInvalidToken)
	})

	t.Run("an expired token is an invalid token, not a server error", func(t *testing.T) {
		claims := baseClaims()
		claims["exp"] = time.Now().Add(-time.Hour).Unix()
		_, err := env.verifier.Verify(ctx, clerkRequest(env.sign(t, jwt.SigningMethodRS256, claims)))
		require.ErrorIs(t, err, ErrInvalidToken,
			"the callback maps ErrInvalidToken to a 401; a raw jwt error would fall through to a 500")
		require.ErrorIs(t, err, jwt.ErrTokenExpired)
	})

	t.Run("a token without a subject is rejected", func(t *testing.T) {
		claims := baseClaims()
		delete(claims, "sub")
		_, err := env.verifier.Verify(ctx, clerkRequest(env.sign(t, jwt.SigningMethodRS256, claims)))
		require.ErrorIs(t, err, ErrInvalidToken)
	})

	t.Run("no credential at all", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/api/auth/login/clerk/callback", nil)
		_, err := env.verifier.Verify(ctx, r)
		require.ErrorIs(t, err, ErrNoSession)
	})

	t.Run("an unverified email claim is carried through as unverified", func(t *testing.T) {
		claims := baseClaims()
		claims["email_verified"] = false

		identity, err := env.verifier.Verify(ctx, clerkRequest(env.sign(t, jwt.SigningMethodRS256, claims)))
		require.NoError(t, err)
		assert.False(t, identity.EmailVerified,
			"the exchange links by email only on a verified claim, so this must not be upgraded here")
	})

	t.Run("a missing email_verified claim is not verified", func(t *testing.T) {
		claims := baseClaims()
		delete(claims, "email_verified")

		identity, err := env.verifier.Verify(ctx, clerkRequest(env.sign(t, jwt.SigningMethodRS256, claims)))
		require.NoError(t, err)
		assert.False(t, identity.EmailVerified)
	})
}

// TestClerkVerifyActClaim covers the impersonation claim. Whether the live token
// template emits `act` is not something we control, so its absence must mean
// "not impersonated" and never a failed login.
func TestClerkVerifyActClaim(t *testing.T) {
	t.Parallel()
	env := newClerkEnv(t)
	ctx := context.Background()

	claims := jwt.MapClaims{
		"iss": clerkIssuer, "sub": "user_act",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"email": "act@example.com", "email_verified": true,
	}

	t.Run("present", func(t *testing.T) {
		withAct := jwt.MapClaims{}
		for k, v := range claims {
			withAct[k] = v
		}
		withAct["act"] = map[string]any{"sub": "user_support_engineer"}

		identity, err := env.verifier.Verify(ctx, clerkRequest(env.sign(t, jwt.SigningMethodRS256, withAct)))
		require.NoError(t, err)
		require.NotNil(t, identity.Actor)
		assert.Equal(t, "user_support_engineer", identity.Actor.Subject)
	})

	t.Run("absent", func(t *testing.T) {
		identity, err := env.verifier.Verify(ctx, clerkRequest(env.sign(t, jwt.SigningMethodRS256, claims)))
		require.NoError(t, err)
		assert.Nil(t, identity.Actor)
	})
}

func clerkUser(id, email string, verified bool) clerk.User {
	primary := "email_primary"
	status := "unverified"
	if verified {
		status = "verified"
	}
	return clerk.User{
		ID:                    id,
		PrimaryEmailAddressID: &primary,
		EmailAddresses: []*clerk.EmailAddress{
			{ID: primary, EmailAddress: email, Verification: &clerk.Verification{Status: status}},
		},
		FirstName: clerk.String("John"),
		LastName:  clerk.String("Doe"),
	}
}

func marshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}

// TestClerkWebhookResolvesThroughIdentities is the ported webhook suite: the
// assertions are unchanged, but every lookup now goes through admin_identities
// instead of the dropped admins.external_id column.
func TestClerkWebhookResolvesThroughIdentities(t *testing.T) {
	t.Parallel()
	env := newClerkEnv(t)
	ctx := context.Background()

	t.Run("user.created provisions an admin reachable by identity", func(t *testing.T) {
		require.NoError(t, env.verifier.handleUserCreated(ctx, marshal(t, clerkUser("user_created", "created@example.com", true))))

		identity, err := env.mgmt.GetAdminIdentity(ctx, clerkIssuer, "user_created")
		require.NoError(t, err)

		admin, err := env.mgmt.GetAdmin(ctx, identity.AdminID)
		require.NoError(t, err)
		assert.Equal(t, "created@example.com", admin.Email)
		assert.Equal(t, "John", *admin.FirstName)
		assert.Equal(t, "Doe", *admin.LastName)
		assert.Equal(t, "owner", admin.Role)
	})

	t.Run("user.created is idempotent", func(t *testing.T) {
		require.NoError(t, env.verifier.handleUserCreated(ctx, marshal(t, clerkUser("user_twice", "twice@example.com", true))))

		first, err := env.mgmt.GetAdminIdentity(ctx, clerkIssuer, "user_twice")
		require.NoError(t, err)

		// A repeat delivery must not overwrite the admin nor create a second one.
		require.NoError(t, env.verifier.handleUserCreated(ctx, marshal(t, clerkUser("user_twice", "renamed@example.com", true))))

		second, err := env.mgmt.GetAdminIdentity(ctx, clerkIssuer, "user_twice")
		require.NoError(t, err)
		assert.Equal(t, first.AdminID, second.AdminID)

		admin, err := env.mgmt.GetAdmin(ctx, second.AdminID)
		require.NoError(t, err)
		assert.Equal(t, "twice@example.com", admin.Email)
	})

	t.Run("user.created without an email is refused", func(t *testing.T) {
		err := env.verifier.handleUserCreated(ctx, marshal(t, clerk.User{ID: "user_no_email"}))
		require.ErrorIs(t, err, ErrInvalidEmail)
	})

	t.Run("user.updated rewrites the admin record", func(t *testing.T) {
		require.NoError(t, env.verifier.handleUserCreated(ctx, marshal(t, clerkUser("user_update", "before@example.com", true))))

		updated := clerkUser("user_update", "after@example.com", true)
		updated.FirstName = clerk.String("New")
		require.NoError(t, env.verifier.handleUserUpdated(ctx, marshal(t, updated)))

		identity, err := env.mgmt.GetAdminIdentity(ctx, clerkIssuer, "user_update")
		require.NoError(t, err)
		admin, err := env.mgmt.GetAdmin(ctx, identity.AdminID)
		require.NoError(t, err)
		assert.Equal(t, "after@example.com", admin.Email)
		assert.Equal(t, "New", *admin.FirstName)
	})

	t.Run("user.updated for an unknown user is a no-op", func(t *testing.T) {
		require.NoError(t, env.verifier.handleUserUpdated(ctx, marshal(t, clerkUser("user_unknown", "unknown@example.com", true))))

		_, err := env.mgmt.GetAdminIdentity(ctx, clerkIssuer, "user_unknown")
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("user.deleted removes the admin", func(t *testing.T) {
		require.NoError(t, env.verifier.handleUserCreated(ctx, marshal(t, clerkUser("user_delete", "delete@example.com", true))))

		identity, err := env.mgmt.GetAdminIdentity(ctx, clerkIssuer, "user_delete")
		require.NoError(t, err)

		require.NoError(t, env.verifier.handleUserDeleted(ctx, marshal(t, map[string]string{"id": "user_delete"})))

		_, err = env.mgmt.GetAdmin(ctx, identity.AdminID)
		require.Error(t, err)
	})

	t.Run("user.deleted for an unknown user is a no-op", func(t *testing.T) {
		require.NoError(t, env.verifier.handleUserDeleted(ctx, marshal(t, map[string]string{"id": "user_never_existed"})))
	})

	t.Run("a webhook reaches an admin whose identity is still on the legacy issuer", func(t *testing.T) {
		orgID, err := env.mgmt.CreateOrganization(ctx, "Legacy Webhook Org")
		require.NoError(t, err)
		adminID, err := env.mgmt.CreateAdmin(ctx, management.Admin{
			OrganizationID: orgID, Email: "legacy-webhook@example.com", Role: "owner",
		})
		require.NoError(t, err)
		_, err = env.mgmt.CreateAdminIdentity(ctx, management.AdminIdentity{
			AdminID:  adminID,
			Provider: management.IdentityProviderClerk,
			Issuer:   management.LegacyExternalIDIssuer,
			Subject:  "user_legacy_webhook",
		})
		require.NoError(t, err)

		require.NoError(t, env.verifier.handleUserDeleted(ctx, marshal(t, map[string]string{"id": "user_legacy_webhook"})))

		_, err = env.mgmt.GetAdmin(ctx, adminID)
		require.Error(t, err, "an admin who has not logged in since the migration is still reachable")
	})
}

func TestClerkWebhookNotConfigured(t *testing.T) {
	t.Parallel()
	verifier, err := NewClerk(config.ClerkAuth{SecretKey: "sk_test_xxx"}, nil, zaptest.NewLogger(t), nil, nil)
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodPost, "/api/auth/clerk/webhook", nil)
	err = verifier.Webhook(context.Background(), r)
	require.Error(t, err)
	require.Contains(t, err.Error(), "webhook client not configured")
}

func TestClerkPrimaryEmail(t *testing.T) {
	t.Parallel()

	primary := "email_123"
	other := "email_456"

	tests := map[string]struct {
		user         clerk.User
		wantEmail    string
		wantVerified bool
	}{
		"verified primary": {
			user: clerk.User{
				PrimaryEmailAddressID: &primary,
				EmailAddresses: []*clerk.EmailAddress{
					{ID: other, EmailAddress: "other@example.com"},
					{ID: primary, EmailAddress: "primary@example.com", Verification: &clerk.Verification{Status: "verified"}},
				},
			},
			wantEmail: "primary@example.com", wantVerified: true,
		},
		"unverified primary is not treated as verified": {
			user: clerk.User{
				PrimaryEmailAddressID: &primary,
				EmailAddresses: []*clerk.EmailAddress{
					{ID: primary, EmailAddress: "primary@example.com", Verification: &clerk.Verification{Status: "unverified"}},
				},
			},
			wantEmail: "primary@example.com", wantVerified: false,
		},
		"no verification block at all": {
			user: clerk.User{
				PrimaryEmailAddressID: &primary,
				EmailAddresses: []*clerk.EmailAddress{
					{ID: primary, EmailAddress: "primary@example.com"},
				},
			},
			wantEmail: "primary@example.com", wantVerified: false,
		},
		"no primary email id": {
			user: clerk.User{EmailAddresses: []*clerk.EmailAddress{{ID: primary, EmailAddress: "x@example.com"}}},
		},
		"primary id not present": {
			user: clerk.User{
				PrimaryEmailAddressID: &other,
				EmailAddresses:        []*clerk.EmailAddress{{ID: primary, EmailAddress: "x@example.com"}},
			},
		},
		"no addresses": {
			user: clerk.User{PrimaryEmailAddressID: &primary, EmailAddresses: []*clerk.EmailAddress{}},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			email, verified := primaryEmail(tc.user)
			assert.Equal(t, tc.wantEmail, email)
			assert.Equal(t, tc.wantVerified, verified)
		})
	}
}
