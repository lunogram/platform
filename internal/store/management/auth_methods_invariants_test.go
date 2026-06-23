package management

import (
	"context"
	"net/http"
	"testing"

	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuthMethodsStoreInvariants covers the security- and correctness-relevant
// invariants of the auth-method store that are not exercised by the happy-path
// suite: global trusted-issuer uniqueness, the session default TTL, and the
// nil-vs-empty grant update semantics.
func TestAuthMethodsStoreInvariants(t *testing.T) {
	t.Parallel()
	db := NewContainerStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Invariants Org")
	require.NoError(t, err)

	// Two distinct projects to prove uniqueness is enforced globally, not
	// per-project: a trusted issuer is resolved by `iss` alone with no project
	// context, so a duplicate must never authenticate against another project.
	projectA, err := db.CreateProject(ctx, Project{
		OrganizationID: &orgID,
		Name:           "Invariants Project A",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)
	projectB, err := db.CreateProject(ctx, Project{
		OrganizationID: &orgID,
		Name:           "Invariants Project B",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	t.Run("trusted_issuer issuer is globally unique across projects", func(t *testing.T) {
		const iss = "https://shared.example"

		first, err := db.CreateAuthMethod(ctx, projectA, CreateAuthMethodInput{
			Type: MethodTypeTrustedIssuer,
			Name: "first idp",
			Role: "support",
			TrustedIssuer: &TrustedIssuer{
				JWKSURL: "https://shared.example/jwks.json",
				Issuer:  iss,
			},
		})
		require.NoError(t, err)
		require.NotNil(t, first)

		// A second ACTIVE method reusing the same iss must be rejected, even in
		// a different project.
		_, err = db.CreateAuthMethod(ctx, projectB, CreateAuthMethodInput{
			Type: MethodTypeTrustedIssuer,
			Name: "duplicate idp",
			Role: "support",
			TrustedIssuer: &TrustedIssuer{
				JWKSURL: "https://shared.example/other-jwks.json",
				Issuer:  iss,
			},
		})
		require.Error(t, err)
		assert.Equal(t, http.StatusConflict, problem.GetStatus(err),
			"a duplicate active iss must be a conflict")

		// The conflict must not have persisted a row in project B.
		methods, total, err := db.ListAuthMethods(ctx, projectB, store.Pagination{Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, 0, total)
		assert.Empty(t, methods)
	})

	t.Run("session TTL defaults to 900s when unset", func(t *testing.T) {
		// TTL nil: no Session block at all.
		nilTTL, err := db.CreateAuthMethod(ctx, projectA, CreateAuthMethodInput{
			Type: MethodTypeSession,
			Name: "default-ttl sessions (nil)",
			Role: "client",
		})
		require.NoError(t, err)

		got, err := db.GetAuthMethod(ctx, projectA, nilTTL.ID)
		require.NoError(t, err)
		require.NotNil(t, got.Session)
		assert.Equal(t, 900, got.Session.TTLSeconds)

		// TTL zero: Session present but TTLSeconds == 0.
		zeroTTL, err := db.CreateAuthMethod(ctx, projectA, CreateAuthMethodInput{
			Type:    MethodTypeSession,
			Name:    "default-ttl sessions (zero)",
			Role:    "client",
			Session: &Session{TTLSeconds: 0},
		})
		require.NoError(t, err)

		got, err = db.GetAuthMethod(ctx, projectA, zeroTTL.ID)
		require.NoError(t, err)
		require.NotNil(t, got.Session)
		assert.Equal(t, 900, got.Session.TTLSeconds)
	})

	t.Run("update grants: nil leaves unchanged, empty clears", func(t *testing.T) {
		created, err := db.CreateAuthMethod(ctx, projectA, CreateAuthMethodInput{
			Type: MethodTypeAPIKey,
			Name: "grant-update key",
			Role: "client",
			Grants: []Grant{
				{Resource: "events", Verb: "create"},
				{Resource: "users", Verb: "update"},
			},
		})
		require.NoError(t, err)

		// Grants == nil: an update that touches another field must leave the
		// existing grant set intact.
		require.NoError(t, db.UpdateAuthMethod(ctx, projectA, created.ID, UpdateAuthMethodInput{
			Role:   ptr.To("editor"),
			Grants: nil,
		}))

		got, err := db.GetAuthMethod(ctx, projectA, created.ID)
		require.NoError(t, err)
		assert.Equal(t, "editor", got.Role)
		assert.ElementsMatch(t, []Grant{
			{Resource: "events", Verb: "create"},
			{Resource: "users", Verb: "update"},
		}, got.Grants, "nil grants must leave the set unchanged")

		// Grants == []Grant{}: an explicit empty slice clears the set.
		require.NoError(t, db.UpdateAuthMethod(ctx, projectA, created.ID, UpdateAuthMethodInput{
			Grants: []Grant{},
		}))

		got, err = db.GetAuthMethod(ctx, projectA, created.ID)
		require.NoError(t, err)
		assert.Empty(t, got.Grants, "an empty grant slice must clear the set")
	})
}
