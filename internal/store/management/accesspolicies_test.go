package management

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccessPoliciesStore(t *testing.T) {
	t.Parallel()
	db := NewContainerStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "AP Org")
	require.NoError(t, err)
	projectID, err := db.CreateProject(ctx, Project{
		OrganizationID: &orgID,
		Name:           "AP Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	t.Run("creates and reads back an api_key policy", func(t *testing.T) {
		created, err := db.CreateAccessPolicy(ctx, projectID, CreateAccessPolicyInput{
			Type:  PolicyTypeAPIKey,
			Name:  "public ingest key",
			Scope: ptr.To("public"),
			Role:  "client",
		})
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, created.ID)
		assert.Equal(t, PolicyTypeAPIKey, created.Type)
		assert.Equal(t, "client", created.Role)
		// The full secret is generated and shown exactly once, prefixed pk_.
		require.NotNil(t, created.Secret)
		assert.True(t, strings.HasPrefix(*created.Secret, "pk_"), "public key should be prefixed pk_")
		require.NotNil(t, created.SecretPrefix)
		assert.Empty(t, created.Grants)
		assert.Nil(t, created.IssuerConfig)

		got, err := db.GetAccessPolicy(ctx, projectID, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, got.ID)
		assert.Equal(t, "public ingest key", got.Name)
		// Reads never expose the plaintext, only the prefix.
		assert.Nil(t, got.Secret)
		require.NotNil(t, got.SecretPrefix)

		// The api_key policy must surface through the backward-compatible
		// project_api_keys view that the legacy ApiKeysStore reads.
		keys, _, err := db.ListApiKeys(ctx, projectID, store.Pagination{Limit: 10})
		require.NoError(t, err)
		require.Len(t, keys, 1)
		assert.Equal(t, created.ID, keys[0].ID)
	})

	t.Run("round-trips typed grants and issuer config for a trusted_issuer policy", func(t *testing.T) {
		created, err := db.CreateAccessPolicy(ctx, projectID, CreateAccessPolicyInput{
			Type: PolicyTypeTrustedIssuer,
			Name: "acme idp",
			Role: "support",
			Grants: []Grant{
				{Resource: "inbox", Verb: "read"},
				{Resource: "users", Verb: "read"},
			},
			IssuerConfig: &IssuerConfig{
				JWKSURL:      "https://acme.example/.well-known/jwks.json",
				Issuer:       "https://acme.example",
				Audience:     "lunogram",
				SubjectClaim: "sub",
			},
		})
		require.NoError(t, err)
		assert.Nil(t, created.Secret)

		got, err := db.GetAccessPolicy(ctx, projectID, created.ID)
		require.NoError(t, err)
		assert.Equal(t, PolicyTypeTrustedIssuer, got.Type)
		assert.Equal(t, []Grant{{Resource: "inbox", Verb: "read"}, {Resource: "users", Verb: "read"}}, got.Grants)
		require.NotNil(t, got.IssuerConfig)
		assert.Equal(t, "https://acme.example/.well-known/jwks.json", got.IssuerConfig.JWKSURL)
		assert.Equal(t, "sub", got.IssuerConfig.SubjectClaim)

		// A non-api_key policy must NOT leak into the legacy view.
		keys, _, err := db.ListApiKeys(ctx, projectID, store.Pagination{Limit: 10})
		require.NoError(t, err)
		for _, k := range keys {
			assert.NotEqual(t, created.ID, k.ID, "trusted_issuer policy must not appear in project_api_keys view")
		}
	})

	t.Run("round-trips a session policy config", func(t *testing.T) {
		created, err := db.CreateAccessPolicy(ctx, projectID, CreateAccessPolicyInput{
			Type:          PolicyTypeSession,
			Name:          "web sessions",
			Role:          "client",
			SessionConfig: &SessionConfig{TTL: 15 * time.Minute, Role: "client"},
		})
		require.NoError(t, err)

		got, err := db.GetAccessPolicy(ctx, projectID, created.ID)
		require.NoError(t, err)
		require.NotNil(t, got.SessionConfig)
		assert.Equal(t, 15*time.Minute, got.SessionConfig.TTL)
	})

	t.Run("lists with total count and updates role + grants", func(t *testing.T) {
		policies, total, err := db.ListAccessPolicies(ctx, projectID, store.Pagination{Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, 3, total)
		assert.Len(t, policies, 3)

		target := policies[0]
		err = db.UpdateAccessPolicy(ctx, projectID, target.ID, UpdateAccessPolicyInput{
			Role:   ptr.To("editor"),
			Grants: []Grant{{Resource: "campaigns", Verb: "read"}},
		})
		require.NoError(t, err)

		updated, err := db.GetAccessPolicy(ctx, projectID, target.ID)
		require.NoError(t, err)
		assert.Equal(t, "editor", updated.Role)
		assert.Equal(t, []Grant{{Resource: "campaigns", Verb: "read"}}, updated.Grants)
	})

	t.Run("soft deletes", func(t *testing.T) {
		created, err := db.CreateAccessPolicy(ctx, projectID, CreateAccessPolicyInput{
			Type: PolicyTypeAPIKey,
			Name: "to delete",
			Role: "support",
		})
		require.NoError(t, err)

		require.NoError(t, db.DeleteAccessPolicy(ctx, projectID, created.ID))

		_, err = db.GetAccessPolicy(ctx, projectID, created.ID)
		assert.True(t, errors.Is(err, store.ErrNoRows), "deleted policy should not be found")
	})
}
