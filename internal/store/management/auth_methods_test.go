package management

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthMethodsStore(t *testing.T) {
	t.Parallel()
	db := NewContainerStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "AM Org")
	require.NoError(t, err)
	projectID, err := db.CreateProject(ctx, Project{
		OrganizationID: &orgID,
		Name:           "AM Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	t.Run("creates an api_key method with grants, secret shown once", func(t *testing.T) {
		created, err := db.CreateAuthMethod(ctx, projectID, CreateAuthMethodInput{
			Type: MethodTypeAPIKey,
			Name: "web key",
			Role: "client",
			Grants: []Grant{
				{Resource: "events", Verb: "create"},
				{Resource: "users", Verb: "update"},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, created.Secret)
		assert.True(t, strings.HasPrefix(*created.Secret, "sk_"))

		got, err := db.GetAuthMethod(ctx, projectID, created.ID)
		require.NoError(t, err)
		assert.Nil(t, got.Secret, "reads never expose the plaintext")
		assert.Equal(t, SubjectScopeAll, got.SubjectScope, "defaults to all")
		assert.ElementsMatch(t, created.Grants, got.Grants)
		assert.Len(t, got.Grants, 2)
	})

	t.Run("resolves a trusted_issuer by its issuer", func(t *testing.T) {
		created, err := db.CreateAuthMethod(ctx, projectID, CreateAuthMethodInput{
			Type: MethodTypeTrustedIssuer,
			Name: "lookup idp",
			Role: "support",
			TrustedIssuer: &TrustedIssuer{
				JWKSURL: "https://lookup.example/jwks.json",
				Issuer:  "https://lookup.example",
			},
		})
		require.NoError(t, err)

		resolved, err := db.GetTrustedIssuerByIssuer(ctx, "https://lookup.example")
		require.NoError(t, err)
		assert.Equal(t, created.ID, resolved.ID)
		assert.Equal(t, projectID, resolved.ProjectID)
		assert.Equal(t, orgID, resolved.OrganizationID)
		require.NotNil(t, resolved.JWKSURL)
		assert.Equal(t, "https://lookup.example/jwks.json", *resolved.JWKSURL)
		assert.Equal(t, "sub", resolved.SubjectClaim)

		_, err = db.GetTrustedIssuerByIssuer(ctx, "https://unknown.example")
		assert.Error(t, err)
	})

	t.Run("creates a trusted_issuer method", func(t *testing.T) {
		created, err := db.CreateAuthMethod(ctx, projectID, CreateAuthMethodInput{
			Type:         MethodTypeTrustedIssuer,
			Name:         "acme idp",
			Role:         "support",
			SubjectScope: SubjectScopeOwn,
			TrustedIssuer: &TrustedIssuer{
				JWKSURL: "https://acme.example/jwks.json",
				Issuer:  "https://acme.example",
			},
		})
		require.NoError(t, err)
		assert.Nil(t, created.Secret)

		got, err := db.GetAuthMethod(ctx, projectID, created.ID)
		require.NoError(t, err)
		require.NotNil(t, got.TrustedIssuer)
		assert.Equal(t, "https://acme.example/jwks.json", got.TrustedIssuer.JWKSURL)
		assert.Equal(t, "sub", got.TrustedIssuer.SubjectClaim, "defaults to sub")
		assert.Equal(t, SubjectScopeOwn, got.SubjectScope)
		assert.Nil(t, got.Session)
	})

	t.Run("rejects a trusted_issuer with neither jwks nor cert (DB CHECK)", func(t *testing.T) {
		_, err := db.CreateAuthMethod(ctx, projectID, CreateAuthMethodInput{
			Type:          MethodTypeTrustedIssuer,
			Name:          "bad idp",
			Role:          "support",
			TrustedIssuer: &TrustedIssuer{Issuer: "https://x"},
		})
		assert.Error(t, err)
	})

	t.Run("creates a session method", func(t *testing.T) {
		created, err := db.CreateAuthMethod(ctx, projectID, CreateAuthMethodInput{
			Type:    MethodTypeSession,
			Name:    "web sessions",
			Role:    "client",
			Session: &Session{TTLSeconds: 600},
		})
		require.NoError(t, err)

		got, err := db.GetAuthMethod(ctx, projectID, created.ID)
		require.NoError(t, err)
		require.NotNil(t, got.Session)
		assert.Equal(t, 600, got.Session.TTLSeconds)

		// Resolvable as a session policy for minting/verifying tokens.
		sess, err := db.GetSessionAuthMethod(created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, sess.ID)
		assert.Equal(t, projectID, sess.ProjectID)
		assert.Equal(t, orgID, sess.OrganizationID)
		assert.Equal(t, 600, sess.TTLSeconds)

		// An api_key method is not a session method.
		apiKeyMethod, err := db.CreateAuthMethod(ctx, projectID, CreateAuthMethodInput{Type: MethodTypeAPIKey, Name: "k", Role: "support"})
		require.NoError(t, err)
		_, err = db.GetSessionAuthMethod(apiKeyMethod.ID)
		assert.Error(t, err)
	})

	t.Run("lists, updates role + grants, soft deletes", func(t *testing.T) {
		methods, total, err := db.ListAuthMethods(ctx, projectID, store.Pagination{Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, 5, total)
		assert.Len(t, methods, 5)

		// Find the api_key method and rewrite its scope.
		var target AuthMethod
		for _, m := range methods {
			if m.Type == MethodTypeAPIKey {
				target = m
			}
		}
		require.NoError(t, db.UpdateAuthMethod(ctx, projectID, target.ID, UpdateAuthMethodInput{
			Role:         ptr.To("editor"),
			SubjectScope: ptr.To(SubjectScopeOwn),
			Grants:       []Grant{{Resource: "campaigns", Verb: "read"}},
		}))

		updated, err := db.GetAuthMethod(ctx, projectID, target.ID)
		require.NoError(t, err)
		assert.Equal(t, "editor", updated.Role)
		assert.Equal(t, SubjectScopeOwn, updated.SubjectScope)
		assert.Equal(t, []Grant{{Resource: "campaigns", Verb: "read"}}, updated.Grants)

		require.NoError(t, db.DeleteAuthMethod(ctx, projectID, target.ID))
		_, err = db.GetAuthMethod(ctx, projectID, target.ID)
		assert.True(t, errors.Is(err, store.ErrNoRows))
	})

	t.Run("persists per-grant create constraints and prunes empty lists", func(t *testing.T) {
		created, err := db.CreateAuthMethod(ctx, projectID, CreateAuthMethodInput{
			Type:   MethodTypeAPIKey,
			Name:   "constrained key",
			Role:   "client",
			Grants: []Grant{{Resource: "events", Verb: "create"}},
			GrantConstraints: GrantConstraints{
				"events": {"purchase", "signup"},
				// An empty list carries no restriction and must not be stored.
				"subscriptions": {},
			},
		})
		require.NoError(t, err)

		got, err := db.GetAuthMethod(ctx, projectID, created.ID)
		require.NoError(t, err)
		assert.Equal(t, GrantConstraints{"events": {"purchase", "signup"}}, got.GrantConstraints)

		// The focused enforcement read surfaces the constrained resources.
		constraints, err := db.GrantConstraints(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, []string{"purchase", "signup"}, constraints["events"])
		assert.True(t, constraints.Permits("events", "purchase"))
		assert.False(t, constraints.Permits("events", "refund"), "a name off the allow-list is rejected")
		assert.True(t, constraints.Permits("subscriptions", "anything"), "an unconstrained resource is unrestricted")

		// A missing method (revoked mid-request) must fail closed: the read
		// returns ErrNoRows rather than an empty (unrestricted) constraint set.
		_, err = db.GrantConstraints(ctx, uuid.New())
		assert.True(t, errors.Is(err, store.ErrNoRows), "a vanished method surfaces an error, not unrestricted")

		// A non-nil constraint map replaces the whole set; an empty map clears it.
		require.NoError(t, db.UpdateAuthMethod(ctx, projectID, created.ID, UpdateAuthMethodInput{
			GrantConstraints: ptr.To(GrantConstraints{}),
		}))
		cleared, err := db.GetAuthMethod(ctx, projectID, created.ID)
		require.NoError(t, err)
		assert.Empty(t, cleared.GrantConstraints)

		constraints, err = db.GrantConstraints(ctx, created.ID)
		require.NoError(t, err)
		assert.True(t, constraints.Permits("events", "purchase"), "a cleared constraint set is unrestricted")
	})
}
