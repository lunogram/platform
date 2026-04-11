package management

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectPushProvidersStore(t *testing.T) {
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

	createProvider := func(t *testing.T, name string) uuid.UUID {
		t.Helper()
		providerID, err := db.CreateProvider(ctx, Provider{
			ProjectID: projectID,
			Module:    "fcm",
			Channels:  Channels{"push"},
			Data:      json.RawMessage(`{}`),
			Name:      name,
		})
		require.NoError(t, err)
		return providerID
	}

	t.Run("upsert creates new push provider", func(t *testing.T) {
		providerID := createProvider(t, "FCM iOS")

		result, err := db.UpsertProjectPushProvider(ctx, ProjectPushProvider{
			ProjectID:  projectID,
			ProviderID: providerID,
			Platform:   PlatformIOS,
		})
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, result.ID)
		assert.Equal(t, projectID, result.ProjectID)
		assert.Equal(t, providerID, result.ProviderID)
		assert.Equal(t, PlatformIOS, result.Platform)
		assert.False(t, result.CreatedAt.IsZero())
		assert.False(t, result.UpdatedAt.IsZero())
	})

	t.Run("upsert updates existing push provider for same platform", func(t *testing.T) {
		providerA := createProvider(t, "FCM Android A")
		providerB := createProvider(t, "FCM Android B")

		resultA, err := db.UpsertProjectPushProvider(ctx, ProjectPushProvider{
			ProjectID:  projectID,
			ProviderID: providerA,
			Platform:   PlatformAndroid,
		})
		require.NoError(t, err)

		resultB, err := db.UpsertProjectPushProvider(ctx, ProjectPushProvider{
			ProjectID:  projectID,
			ProviderID: providerB,
			Platform:   PlatformAndroid,
		})
		require.NoError(t, err)

		// Same row was updated, so the ID should be the same.
		assert.Equal(t, resultA.ID, resultB.ID)
		assert.Equal(t, providerB, resultB.ProviderID)
	})

	t.Run("upsert with non-existent provider returns error", func(t *testing.T) {
		_, err := db.UpsertProjectPushProvider(ctx, ProjectPushProvider{
			ProjectID:  projectID,
			ProviderID: uuid.New(),
			Platform:   PlatformWeb,
		})
		require.Error(t, err)
		assert.ErrorContains(t, err, "provider not found")
	})

	t.Run("get returns push provider by platform", func(t *testing.T) {
		providerID := createProvider(t, "FCM Web")

		created, err := db.UpsertProjectPushProvider(ctx, ProjectPushProvider{
			ProjectID:  projectID,
			ProviderID: providerID,
			Platform:   PlatformWeb,
		})
		require.NoError(t, err)

		got, err := db.GetProjectPushProvider(ctx, projectID, PlatformWeb)
		require.NoError(t, err)
		assert.Equal(t, created.ID, got.ID)
		assert.Equal(t, providerID, got.ProviderID)
		assert.Equal(t, PlatformWeb, got.Platform)
	})

	t.Run("get returns error for non-existent platform", func(t *testing.T) {
		otherProjectID, err := db.CreateProject(ctx, Project{
			OrganizationID: &orgID,
			Name:           "Empty Project",
			Timezone:       "UTC",
			Locale:         "en",
		})
		require.NoError(t, err)

		_, err = db.GetProjectPushProvider(ctx, otherProjectID, PlatformIOS)
		require.Error(t, err)
	})

	t.Run("list returns all push providers for a project", func(t *testing.T) {
		listProjectID, err := db.CreateProject(ctx, Project{
			OrganizationID: &orgID,
			Name:           "List Project",
			Timezone:       "UTC",
			Locale:         "en",
		})
		require.NoError(t, err)

		providerA := createProvider(t, "List Provider A")
		providerB := createProvider(t, "List Provider B")

		_, err = db.UpsertProjectPushProvider(ctx, ProjectPushProvider{
			ProjectID:  listProjectID,
			ProviderID: providerA,
			Platform:   PlatformAndroid,
		})
		require.NoError(t, err)

		_, err = db.UpsertProjectPushProvider(ctx, ProjectPushProvider{
			ProjectID:  listProjectID,
			ProviderID: providerB,
			Platform:   PlatformIOS,
		})
		require.NoError(t, err)

		providers, err := db.ListProjectPushProviders(ctx, listProjectID)
		require.NoError(t, err)
		assert.Len(t, providers, 2)

		// Results are ordered by platform alphabetically: android < ios.
		assert.Equal(t, PlatformAndroid, providers[0].Platform)
		assert.Equal(t, PlatformIOS, providers[1].Platform)
	})

	t.Run("list returns empty slice for project with no push providers", func(t *testing.T) {
		emptyProjectID, err := db.CreateProject(ctx, Project{
			OrganizationID: &orgID,
			Name:           "Empty List Project",
			Timezone:       "UTC",
			Locale:         "en",
		})
		require.NoError(t, err)

		providers, err := db.ListProjectPushProviders(ctx, emptyProjectID)
		require.NoError(t, err)
		assert.Empty(t, providers)
	})

	t.Run("delete removes push provider", func(t *testing.T) {
		deleteProjectID, err := db.CreateProject(ctx, Project{
			OrganizationID: &orgID,
			Name:           "Delete Project",
			Timezone:       "UTC",
			Locale:         "en",
		})
		require.NoError(t, err)

		providerID := createProvider(t, "Delete Provider")

		_, err = db.UpsertProjectPushProvider(ctx, ProjectPushProvider{
			ProjectID:  deleteProjectID,
			ProviderID: providerID,
			Platform:   PlatformIOS,
		})
		require.NoError(t, err)

		err = db.DeleteProjectPushProvider(ctx, deleteProjectID, PlatformIOS)
		require.NoError(t, err)

		_, err = db.GetProjectPushProvider(ctx, deleteProjectID, PlatformIOS)
		require.Error(t, err)
	})

	t.Run("delete is idempotent for non-existent platform", func(t *testing.T) {
		err := db.DeleteProjectPushProvider(ctx, projectID, "nonexistent")
		require.NoError(t, err)
	})

	t.Run("OAPI conversion", func(t *testing.T) {
		providerID := createProvider(t, "OAPI Provider")

		result, err := db.UpsertProjectPushProvider(ctx, ProjectPushProvider{
			ProjectID:  projectID,
			ProviderID: providerID,
			Platform:   PlatformIOS,
		})
		require.NoError(t, err)

		o := result.OAPI()
		assert.Equal(t, result.ID, o.Id)
		assert.Equal(t, result.ProjectID, o.ProjectId)
		assert.Equal(t, result.ProviderID, o.ProviderId)
		assert.Equal(t, result.CreatedAt, o.CreatedAt)
		assert.Equal(t, result.UpdatedAt, o.UpdatedAt)
	})

	t.Run("OAPI slice conversion", func(t *testing.T) {
		sliceProjectID, err := db.CreateProject(ctx, Project{
			OrganizationID: &orgID,
			Name:           "OAPI Slice Project",
			Timezone:       "UTC",
			Locale:         "en",
		})
		require.NoError(t, err)

		providerID := createProvider(t, "OAPI Slice Provider")

		_, err = db.UpsertProjectPushProvider(ctx, ProjectPushProvider{
			ProjectID:  sliceProjectID,
			ProviderID: providerID,
			Platform:   PlatformAndroid,
		})
		require.NoError(t, err)

		providers, err := db.ListProjectPushProviders(ctx, sliceProjectID)
		require.NoError(t, err)

		oapiSlice := providers.OAPI()
		assert.Len(t, oapiSlice, len(providers))
	})
}
