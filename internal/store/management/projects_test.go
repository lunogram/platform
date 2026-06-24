package management

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectsStore(t *testing.T) {
	t.Parallel()
	db := NewContainerStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	t.Run("creates project", func(t *testing.T) {
		projectID, err := db.CreateProject(ctx, Project{
			OrganizationID: &orgID,
			Name:           "Test Project",
			Timezone:       "UTC",
			Locale:         "en",
		})
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, projectID)

		project, err := db.GetProject(ctx, projectID, nil)
		require.NoError(t, err)
		assert.Equal(t, "Test Project", project.Name)
		assert.Equal(t, "UTC", project.Timezone)
		assert.Equal(t, "en", project.Locale)
	})

	t.Run("updates project", func(t *testing.T) {
		projectID, err := db.CreateProject(ctx, Project{
			OrganizationID: &orgID,
			Name:           "Original Name",
			Timezone:       "UTC",
			Locale:         "en",
		})
		require.NoError(t, err)

		err = db.UpdateProject(ctx, projectID, ProjectUpdate{
			Name:     ptr.To("Updated Name"),
			Timezone: ptr.To("America/New_York"),
			Locale:   ptr.To("en-GB"),
		})
		require.NoError(t, err)

		project, err := db.GetProject(ctx, projectID, nil)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", project.Name)
		assert.Equal(t, "America/New_York", project.Timezone)
		assert.Equal(t, "en-GB", project.Locale)
	})

	t.Run("lists projects by organization", func(t *testing.T) {
		// Create another org with its own projects
		org2ID, err := db.CreateOrganization(ctx, "Another Org")
		require.NoError(t, err)

		_, err = db.CreateProject(ctx, Project{
			OrganizationID: &org2ID,
			Name:           "Project in Org 2",
			Timezone:       "UTC",
			Locale:         "en",
		})
		require.NoError(t, err)

		projects, total, err := db.ListProjects(ctx, org2ID, store.Pagination{Limit: 10, Offset: 0}, "")
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		assert.Len(t, projects, 1)
		assert.Equal(t, "Project in Org 2", projects[0].Name)
	})
}
