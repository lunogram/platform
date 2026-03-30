package subjects

import (
	"testing"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInsertCampaignSend_MultipleBroadcastsSameCampaignAndUser(t *testing.T) {
	t.Parallel()

	state := NewContainerStore(t)
	ctx := graceful.NewContext(t.Context())

	projectID := uuid.New()
	email := "user@test.com"
	userID, err := state.UsersStore.CreateUser(ctx, projectID, &email, nil, []byte(`{}`), nil, nil, nil)
	require.NoError(t, err)

	campaignID := uuid.New()
	broadcastA := uuid.New()
	broadcastB := uuid.New()

	now := time.Now()
	sentState := CampaignSendStateSent
	refType := "test-provider"

	// Insert a send for broadcast A.
	err = state.CampaignSendsStore.InsertCampaignSend(ctx, CampaignSend{
		CampaignID:    campaignID,
		UserID:        userID,
		BroadcastID:   &broadcastA,
		State:         &sentState,
		SentAt:        &now,
		ReferenceType: &refType,
		ReferenceID:   "",
	})
	require.NoError(t, err)

	// Insert a send for broadcast B — same campaign, same user, same
	// empty reference_id. Before the PK fix this would be silently
	// dropped by ON CONFLICT DO NOTHING.
	err = state.CampaignSendsStore.InsertCampaignSend(ctx, CampaignSend{
		CampaignID:    campaignID,
		UserID:        userID,
		BroadcastID:   &broadcastB,
		State:         &sentState,
		SentAt:        &now,
		ReferenceType: &refType,
		ReferenceID:   "",
	})
	require.NoError(t, err)

	// Both broadcasts must have exactly 1 send each.
	countA, err := state.CampaignSendsStore.CountSendsByBroadcastID(ctx, broadcastA)
	require.NoError(t, err)
	assert.Equal(t, 1, countA, "broadcast A should have 1 send")

	countB, err := state.CampaignSendsStore.CountSendsByBroadcastID(ctx, broadcastB)
	require.NoError(t, err)
	assert.Equal(t, 1, countB, "broadcast B should have 1 send")
}

func TestInsertCampaignSend_NilBroadcastID(t *testing.T) {
	t.Parallel()

	state := NewContainerStore(t)
	ctx := graceful.NewContext(t.Context())

	projectID := uuid.New()
	email := "journey-user@test.com"
	userID, err := state.UsersStore.CreateUser(ctx, projectID, &email, nil, []byte(`{}`), nil, nil, nil)
	require.NoError(t, err)

	campaignID := uuid.New()
	now := time.Now()
	sentState := CampaignSendStateSent
	refType := "test-provider"

	// A non-broadcast send (e.g. journey trigger) has a nil BroadcastID.
	err = state.CampaignSendsStore.InsertCampaignSend(ctx, CampaignSend{
		CampaignID:    campaignID,
		UserID:        userID,
		BroadcastID:   nil,
		State:         &sentState,
		SentAt:        &now,
		ReferenceType: &refType,
		ReferenceID:   "resp-1",
	})
	require.NoError(t, err)

	// The zero UUID is used internally for nil broadcast IDs.
	count, err := state.CampaignSendsStore.CountSendsByBroadcastID(ctx, uuid.Nil)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestInsertCampaignSend_DuplicateWithinSameBroadcast(t *testing.T) {
	t.Parallel()

	state := NewContainerStore(t)
	ctx := graceful.NewContext(t.Context())

	projectID := uuid.New()
	email := "dedup@test.com"
	userID, err := state.UsersStore.CreateUser(ctx, projectID, &email, nil, []byte(`{}`), nil, nil, nil)
	require.NoError(t, err)

	campaignID := uuid.New()
	broadcastID := uuid.New()
	now := time.Now()
	sentState := CampaignSendStateSent
	refType := "test-provider"

	send := CampaignSend{
		CampaignID:    campaignID,
		UserID:        userID,
		BroadcastID:   &broadcastID,
		State:         &sentState,
		SentAt:        &now,
		ReferenceType: &refType,
		ReferenceID:   "",
	}

	// First insert succeeds.
	err = state.CampaignSendsStore.InsertCampaignSend(ctx, send)
	require.NoError(t, err)

	// Duplicate within the same broadcast is silently ignored (idempotent).
	err = state.CampaignSendsStore.InsertCampaignSend(ctx, send)
	require.NoError(t, err)

	count, err := state.CampaignSendsStore.CountSendsByBroadcastID(ctx, broadcastID)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "duplicate insert should not create a second row")
}
