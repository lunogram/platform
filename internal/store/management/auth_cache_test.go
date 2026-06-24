package management

import (
	"fmt"
	"testing"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/store"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// newCachedContainerStore builds a management State backed by a real Postgres
// and a real Redis, with the auth read-through caches enabled.
func newCachedContainerStore(t *testing.T) *State {
	t.Helper()

	uri := container.RunPostgreSQL(t)
	mgmtURI := container.CreateSchema(t, uri, "authcache")
	require.NoError(t, Migrate(mgmtURI))

	ctx := graceful.NewContext(t.Context())
	db, err := store.Connect(ctx, zaptest.NewLogger(t), mgmtURI)
	require.NoError(t, err)

	ropts, err := goredis.ParseURL(container.RunRedis(t))
	require.NoError(t, err)
	rclient := goredis.NewClient(ropts)
	t.Cleanup(func() { _ = rclient.Close() })

	prefix := fmt.Sprintf("authcachetest:%d:", time.Now().UnixNano())
	return NewState(db, WithRedis(rclient, prefix))
}

func TestAuthCacheReadThroughAndInvalidation(t *testing.T) {
	t.Parallel()
	st := newCachedContainerStore(t)
	ctx := graceful.NewContext(t.Context())

	orgID, err := st.CreateOrganization(ctx, "Cache Org")
	require.NoError(t, err)
	projectID, err := st.CreateProject(ctx, Project{
		OrganizationID: &orgID,
		Name:           "Cache Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	created, err := st.CreateAuthMethod(ctx, projectID, CreateAuthMethodInput{
		Type: MethodTypeAPIKey,
		Name: "backend",
		Role: "support",
	})
	require.NoError(t, err)
	require.NotNil(t, created.Secret)
	secret := *created.Secret

	// Read-through populates the cache.
	got, err := st.GetAPIKeyBySecret(ctx, secret)
	require.NoError(t, err)
	assert.Equal(t, "support", got.Role)

	// A role change invalidates the cached entry, so the next read is fresh
	// rather than the stale "support" value held in Redis.
	require.NoError(t, st.UpdateAuthMethod(ctx, projectID, created.ID, UpdateAuthMethodInput{Role: ptr.To("editor")}))
	got, err = st.GetAPIKeyBySecret(ctx, secret)
	require.NoError(t, err)
	assert.Equal(t, "editor", got.Role, "update must invalidate the cached credential")

	// A delete invalidates too, so the key stops resolving immediately.
	require.NoError(t, st.DeleteAuthMethod(ctx, projectID, created.ID))
	_, err = st.GetAPIKeyBySecret(ctx, secret)
	require.Error(t, err, "delete must invalidate the cached credential")
}
