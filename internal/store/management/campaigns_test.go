package management

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/store"
	"github.com/stretchr/testify/require"
)

// createCampaignTestProject creates an organization and project so campaigns can
// satisfy their project_id foreign key.
func createCampaignTestProject(t *testing.T, ctx context.Context, db *State) uuid.UUID {
	t.Helper()

	orgID, err := db.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	projectID, err := db.CreateProject(ctx, Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	return projectID
}

func TestListCampaigns_ArchivedFilter(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	ctx := context.Background()
	projectID := createCampaignTestProject(t, ctx, db)

	activeID, err := db.CreateCampaign(ctx, Campaign{ProjectID: projectID, Name: "Active Campaign", Channel: "email"})
	require.NoError(t, err)

	archivedID, err := db.CreateCampaign(ctx, Campaign{ProjectID: projectID, Name: "Archived Campaign", Channel: "email"})
	require.NoError(t, err)
	require.NoError(t, db.DeleteCampaign(ctx, projectID, archivedID))

	page := store.Pagination{Limit: 10, Offset: 0}

	// archivedOnly=false returns only active campaigns, with a total that excludes archived ones.
	active, total, err := db.ListCampaigns(ctx, projectID, page, "", false)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, active, 1)
	require.Equal(t, activeID, active[0].ID)
	require.Nil(t, active[0].DeletedAt)

	// archivedOnly=true returns only archived campaigns, with a matching total for pagination.
	archived, total, err := db.ListCampaigns(ctx, projectID, page, "", true)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, archived, 1)
	require.Equal(t, archivedID, archived[0].ID)
	require.NotNil(t, archived[0].DeletedAt)
}

func TestUnarchiveCampaign(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	ctx := context.Background()
	projectID := createCampaignTestProject(t, ctx, db)

	campaignID, err := db.CreateCampaign(ctx, Campaign{ProjectID: projectID, Name: "Restore Me", Channel: "email"})
	require.NoError(t, err)
	require.NoError(t, db.DeleteCampaign(ctx, projectID, campaignID))

	// Restoring an archived campaign clears deleted_at and brings it back to the active list.
	require.NoError(t, db.UnarchiveCampaign(ctx, projectID, campaignID))

	active, total, err := db.ListCampaigns(ctx, projectID, store.Pagination{Limit: 10, Offset: 0}, "", false)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, active, 1)
	require.Equal(t, campaignID, active[0].ID)

	// Unarchiving a campaign that is not archived (already active) reports no rows affected.
	err = db.UnarchiveCampaign(ctx, projectID, campaignID)
	require.ErrorIs(t, err, store.ErrNoRows)

	// Unarchiving a non-existent campaign reports no rows affected.
	err = db.UnarchiveCampaign(ctx, projectID, uuid.New())
	require.ErrorIs(t, err, store.ErrNoRows)
}
