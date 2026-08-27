package consumer

import (
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestBroadcastCompletesOnSentPlusFailed is the regression this whole counter
// split exists for: before `failed` existed, a broadcast whose tail was
// suppressed never reached its total and sat in "sending" forever, because only
// the sent path advanced a counter.
func TestBroadcastCompletesOnSentPlusFailed(t *testing.T) {
	t.Parallel()

	ctx := graceful.NewContext(t.Context())
	mgmtDB, _, _ := teststore.RunPostgreSQL(t)
	mgmt := management.NewState(mgmtDB)

	organizationID, err := mgmt.OrganizationsStore.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projectID, err := mgmt.ProjectsStore.CreateProject(ctx, management.Project{
		OrganizationID: &organizationID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	campaignID, err := mgmt.CampaignsStore.CreateCampaign(ctx, management.Campaign{
		ProjectID: projectID,
		Name:      "Test Campaign",
		Channel:   "sms",
	})
	require.NoError(t, err)

	broadcast, err := mgmt.BroadcastsStore.CreateBroadcast(ctx, management.Broadcast{
		ProjectID:  projectID,
		CampaignID: campaignID,
		ListID:     uuid.New(),
		ListName:   "Test List",
		ListType:   "static",
	})
	require.NoError(t, err)

	require.NoError(t, mgmt.BroadcastsStore.IncrementBroadcastTotal(ctx, projectID, broadcast.ID, 3))
	require.NoError(t, mgmt.BroadcastsStore.TransitionPendingBroadcastToSending(ctx, broadcast.ID))

	handler := &InboxHandler{logger: zaptest.NewLogger(t), mgmt: mgmt}
	userID := uuid.New()
	message := &subjects.InboxMessage{
		ID:          uuid.New(),
		ProjectID:   projectID,
		UserID:      &userID,
		BroadcastID: &broadcast.ID,
	}

	require.NoError(t, handler.completeBroadcastIfDone(ctx, message, broadcastOutcomeSent))
	require.NoError(t, handler.completeBroadcastIfDone(ctx, message, broadcastOutcomeFailed))

	current, err := mgmt.BroadcastsStore.GetBroadcast(ctx, projectID, broadcast.ID)
	require.NoError(t, err)
	require.Equal(t, management.BroadcastStateSending, current.State, "two of three messages have settled")

	require.NoError(t, handler.completeBroadcastIfDone(ctx, message, broadcastOutcomeSent))

	current, err = mgmt.BroadcastsStore.GetBroadcast(ctx, projectID, broadcast.ID)
	require.NoError(t, err)
	require.Equal(t, management.BroadcastStateCompleted, current.State)
	require.Equal(t, 2, current.Sent, "a suppressed message must never be reported as sent")
	require.Equal(t, 1, current.Failed)
	require.Equal(t, 3, current.Total)
}

// TestBroadcastIgnoresMessagesWithoutABroadcast asserts the counters are only
// touched by broadcast members.
func TestBroadcastIgnoresMessagesWithoutABroadcast(t *testing.T) {
	t.Parallel()

	handler := &InboxHandler{logger: zaptest.NewLogger(t)}
	userID := uuid.New()
	message := &subjects.InboxMessage{ID: uuid.New(), ProjectID: uuid.New(), UserID: &userID}

	require.NoError(t, handler.completeBroadcastIfDone(t.Context(), message, broadcastOutcomeFailed))
}

func TestBroadcastProgressSettled(t *testing.T) {
	t.Parallel()

	require.False(t, management.BroadcastProgress{Sent: 0, Failed: 0, Total: 0}.Settled(), "an uncounted broadcast must not settle")
	require.False(t, management.BroadcastProgress{Sent: 1, Failed: 1, Total: 3}.Settled())
	require.True(t, management.BroadcastProgress{Sent: 2, Failed: 1, Total: 3}.Settled())
	require.True(t, management.BroadcastProgress{Sent: 0, Failed: 3, Total: 3}.Settled())
	require.True(t, management.BroadcastProgress{Sent: 4, Failed: 0, Total: 3}.Settled())
}
