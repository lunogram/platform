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

func TestApiKeysStore(t *testing.T) {
	t.Parallel()
	db := NewContainerStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Keys Org")
	require.NoError(t, err)
	projectID, err := db.CreateProject(ctx, Project{
		OrganizationID: &orgID,
		Name:           "Keys Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	t.Run("creates an api key across auth_methods + auth_method_api_keys", func(t *testing.T) {
		created, err := db.CreateApiKey(ctx, projectID, "web key", ScopePublic, "client", ptr.To("desc"))
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(created.Plaintext, "pk_"), "public key should be pk_ prefixed")
		assert.Equal(t, created.Plaintext[:secretPrefixLen], created.SecretPrefix)
		require.NotNil(t, created.Scope)
		assert.Equal(t, ScopePublic, *created.Scope)

		// Reads never expose the plaintext, only the prefix.
		got, err := db.GetApiKey(ctx, projectID, created.ID)
		require.NoError(t, err)
		assert.Empty(t, got.Plaintext)
		assert.Equal(t, created.SecretPrefix, got.SecretPrefix)
		assert.Equal(t, "client", got.Role)

		// Auth resolves the key by hash of the presented plaintext.
		resolved, err := db.GetAPIKeyBySecret(created.Plaintext)
		require.NoError(t, err)
		assert.Equal(t, created.ID.String(), resolved.ID)
		assert.Equal(t, projectID, resolved.ProjectID)
		assert.Equal(t, orgID, resolved.OrganizationID)
		assert.Equal(t, "client", resolved.Role)

		// A wrong secret does not resolve.
		_, err = db.GetAPIKeyBySecret("pk_not_a_real_secret")
		assert.Error(t, err)
	})

	t.Run("lists, updates, and soft deletes", func(t *testing.T) {
		created, err := db.CreateApiKey(ctx, projectID, "backend key", ScopeSecret, "support", nil)
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(created.Plaintext, "sk_"))

		keys, total, err := db.ListApiKeys(ctx, projectID, store.Pagination{Limit: 10})
		require.NoError(t, err)
		assert.Equal(t, 2, total)
		assert.Len(t, keys, 2)

		require.NoError(t, db.UpdateApiKey(ctx, projectID, created.ID, nil, ptr.To("editor"), nil))
		got, err := db.GetApiKey(ctx, projectID, created.ID)
		require.NoError(t, err)
		assert.Equal(t, "editor", got.Role)

		require.NoError(t, db.DeleteApiKey(ctx, projectID, created.ID))
		_, err = db.GetApiKey(ctx, projectID, created.ID)
		assert.True(t, errors.Is(err, store.ErrNoRows))

		// A deleted key no longer authenticates.
		_, err = db.GetAPIKeyBySecret(created.Plaintext)
		assert.Error(t, err)
	})
}
