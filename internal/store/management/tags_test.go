package management

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTagsStore(t *testing.T) {
	t.Parallel()
	db := NewContainerStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	projectID, err := db.CreateProject(ctx, Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	t.Run("creates tag", func(t *testing.T) {
		tagID, err := db.CreateTag(ctx, projectID, "VIP")
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, tagID)

		tag, err := db.GetTag(ctx, projectID, tagID)
		require.NoError(t, err)
		assert.Equal(t, "VIP", tag.Name)
	})

	t.Run("lists tags", func(t *testing.T) {
		_, err := db.CreateTag(ctx, projectID, "Tag A")
		require.NoError(t, err)
		_, err = db.CreateTag(ctx, projectID, "Tag B")
		require.NoError(t, err)

		tags, total, err := db.ListTags(ctx, projectID, store.Pagination{Limit: 10, Offset: 0}, "")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, 2)
		assert.GreaterOrEqual(t, len(tags), 2)
	})
}
