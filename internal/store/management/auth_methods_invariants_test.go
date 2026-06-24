package management

import (
	"context"
	"net/http"
	"testing"

	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuthMethodsStoreInvariants covers the security- and correctness-relevant
// invariants of the auth-method store that are not exercised by the happy-path
// suite: per-project trusted-issuer uniqueness and cross-tenant isolation, the
// session default TTL, and the nil-vs-empty grant update semantics.
func TestAuthMethodsStoreInvariants(t *testing.T) {
	t.Parallel()
	db := NewContainerStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Invariants Org")
	require.NoError(t, err)

	// Two distinct projects to prove issuers are project-scoped: a trusted issuer
	// is resolved by (project, `iss`), so the same `iss` may live in two projects
	// and each must resolve only to its own method — never the other tenant's.
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

	t.Run("trusted_issuer issuer is unique per project but reusable across projects", func(t *testing.T) {
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

		// A second ACTIVE method reusing the same iss in the SAME project is a
		// conflict: resolution is (project, iss)-scoped, so a duplicate within a
		// project would be ambiguous.
		_, err = db.CreateAuthMethod(ctx, projectA, CreateAuthMethodInput{
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
			"a duplicate active iss in the same project must be a conflict")

		// The same iss in a DIFFERENT project is allowed: issuers are project-scoped,
		// so two tenants may each register the same upstream IdP.
		second, err := db.CreateAuthMethod(ctx, projectB, CreateAuthMethodInput{
			Type: MethodTypeTrustedIssuer,
			Name: "other-project idp",
			Role: "support",
			TrustedIssuer: &TrustedIssuer{
				JWKSURL: "https://shared.example/jwks.json",
				Issuer:  iss,
			},
		})
		require.NoError(t, err, "the same iss must be registrable under a different project")
		require.NotNil(t, second)

		// Each project resolves the shared iss to its own method — the cross-tenant
		// isolation this whole change exists to guarantee.
		resolvedA, err := db.GetTrustedIssuer(ctx, projectA, iss)
		require.NoError(t, err)
		assert.Equal(t, first.ID, resolvedA.ID)
		resolvedB, err := db.GetTrustedIssuer(ctx, projectB, iss)
		require.NoError(t, err)
		assert.Equal(t, second.ID, resolvedB.ID)
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
