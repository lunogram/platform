package management

import (
	"context"
	"errors"
	"strings"
	"testing"

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
	})

	t.Run("lists, updates role + grants, soft deletes", func(t *testing.T) {
		methods, total, err := db.ListAuthMethods(ctx, projectID, store.Pagination{Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, methods, 3)

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
}
