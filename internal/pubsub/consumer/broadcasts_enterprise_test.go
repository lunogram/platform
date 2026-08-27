//go:build enterprise

package consumer

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	teststore "github.com/lunogram/platform/internal/store/test"
	wasmProviders "github.com/lunogram/platform/internal/wasm/providers"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

// createTestUser creates a user in the users table and returns its ID.
func createTestUser(t *testing.T, ctx graceful.Context, usrs *subjects.State, projectID uuid.UUID) uuid.UUID {
	t.Helper()
	email := uuid.New().String() + "@test.com"
	userID, err := usrs.UsersStore.CreateUser(ctx, projectID, &email, nil, json.RawMessage(`{}`), nil, nil, nil)
	require.NoError(t, err)
	return userID
}

// setupBroadcastsTest spins up real NATS and PostgreSQL containers and
// returns everything needed by the broadcast handlers.
func setupBroadcastsTest(t *testing.T) (
	mgmtState *management.State,
	usrsState *subjects.State,
	projectID uuid.UUID,
	jet jetstream.JetStream,
	ns Namespace,
) {
	t.Helper()

	ctx := graceful.NewContext(t.Context())

	natsURL := container.RunNATS(t)
	mgmtDB, usrsDB, _ := teststore.RunPostgreSQL(t)

	cfg := config.Node{
		Nats: config.Nats{
			URL: natsURL,
		},
	}

	var err error
	jet, err = pubsub.New(ctx, cfg)
	require.NoError(t, err)

	mgmtState = management.NewState(mgmtDB)
	usrsState = subjects.NewState(usrsDB, zap.NewNop())

	orgID, err := mgmtState.OrganizationsStore.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projectID, err = mgmtState.ProjectsStore.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	ns = testNamespace(t)

	return mgmtState, usrsState, projectID, jet, ns
}

// seedBroadcast creates a provider, campaign, list, and broadcast in the
// database and returns their IDs. The broadcast is created in pending state.
func seedBroadcast(t *testing.T, ctx graceful.Context, mgmt *management.State, usrs *subjects.State, projectID uuid.UUID) (broadcastID, campaignID, listID uuid.UUID) {
	t.Helper()

	// A provider must exist for the channel so the broadcast can resolve one
	// at send time; campaigns no longer reference a provider directly.
	_, err := mgmt.ProvidersStore.CreateProvider(ctx, management.Provider{
		ProjectID: projectID,
		Module:    "test",
		Channels:  management.Channels{"email"},
		Data:      json.RawMessage(`{}`),
		Name:      "Test Provider",
	})
	require.NoError(t, err)

	campaignID, err = mgmt.CampaignsStore.CreateCampaign(ctx, management.Campaign{
		ProjectID: projectID,
		Name:      "Test Campaign",
		Channel:   "email",
	})
	require.NoError(t, err)

	listID, err = usrs.ListsStore.CreateList(ctx, subjects.List{
		ProjectID: projectID,
		Name:      "Test List",
		Type:      "static",
	})
	require.NoError(t, err)

	broadcast, err := mgmt.BroadcastsStore.CreateBroadcast(ctx, management.Broadcast{
		ProjectID:  projectID,
		CampaignID: campaignID,
		ListID:     listID,
		ListName:   "Test List",
		ListType:   "static",
	})
	require.NoError(t, err)

	broadcastID = broadcast.ID
	return broadcastID, campaignID, listID
}

// ----- BroadcastProcessHandler tests -----

func TestBroadcastProcessHandler_Success(t *testing.T) {
	t.Parallel()

	mgmtState, usrsState, projectID, jet, ns := setupBroadcastsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))

	broadcastID, _, _ := seedBroadcast(t, ctx, mgmtState, usrsState, projectID)

	// Transition broadcast to sending (the process handler expects it).
	err = mgmtState.BroadcastsStore.TransitionPendingBroadcastToSending(ctx, broadcastID)
	require.NoError(t, err)

	registry := wasmProviders.NewRegistry(config.WASM{}, zap.NewNop())
	handler := BroadcastProcessHandler(logger, mgmtState, usrsState, registry, pub, ns)

	event := schemas.ProcessBroadcast{
		ProjectID:   projectID,
		BroadcastID: broadcastID,
	}
	err = pub.Publish(ctx, schemas.BroadcastsProcess(projectID, broadcastID), event)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamBroadcasts), ns.Consumer(ConsumerBroadcastsProcess))
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)

	// Verify that a batch message was published to the batch consumer.
	batchConsumer, err := jet.Consumer(ctx, ns.Stream(StreamBroadcasts), ns.Consumer(ConsumerBroadcastsBatch))
	require.NoError(t, err)

	batchMsg, err := batchConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	var batchEvent schemas.ProcessBroadcastBatch
	err = json.Unmarshal(batchMsg.Data(), &batchEvent)
	require.NoError(t, err)

	assert.Equal(t, projectID, batchEvent.ProjectID)
	assert.Equal(t, broadcastID, batchEvent.BroadcastID)
	assert.Equal(t, 0, batchEvent.Offset)
	assert.Equal(t, DefaultBroadcastBatchSize, batchEvent.BatchSize)
	assert.Equal(t, 0, batchEvent.Processed)
}

func TestBroadcastProcessHandler_NotSendingState(t *testing.T) {
	t.Parallel()

	mgmtState, usrsState, projectID, jet, ns := setupBroadcastsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))

	broadcastID, _, _ := seedBroadcast(t, ctx, mgmtState, usrsState, projectID)
	// broadcast is still in "pending" state — do NOT transition to sending

	registry := wasmProviders.NewRegistry(config.WASM{}, zap.NewNop())
	handler := BroadcastProcessHandler(logger, mgmtState, usrsState, registry, pub, ns)

	event := schemas.ProcessBroadcast{
		ProjectID:   projectID,
		BroadcastID: broadcastID,
	}
	err = pub.Publish(ctx, schemas.BroadcastsProcess(projectID, broadcastID), event)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamBroadcasts), ns.Consumer(ConsumerBroadcastsProcess))
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.Error(t, err)
	assert.True(t, IsPermanent(err), "error should be permanent because the broadcast is not in sending state")
}

// TestBroadcastProcessHandler_CampaignNoProvider verifies that a broadcast
// whose campaign has no provider configured for its channel is still processed
// into batches. Providers are owned by the project and resolved per message at
// dispatch time, so the process handler no longer inspects them.
func TestBroadcastProcessHandler_CampaignNoProvider(t *testing.T) {
	t.Parallel()

	mgmtState, usrsState, projectID, jet, ns := setupBroadcastsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))

	// Create a campaign with no provider configured for its channel.
	campaignID, err := mgmtState.CampaignsStore.CreateCampaign(ctx, management.Campaign{
		ProjectID: projectID,
		Name:      "No Provider Campaign",
		Channel:   "email",
	})
	require.NoError(t, err)

	listID, err := usrsState.ListsStore.CreateList(ctx, subjects.List{
		ProjectID: projectID,
		Name:      "Test List",
		Type:      "static",
	})
	require.NoError(t, err)

	broadcast, err := mgmtState.BroadcastsStore.CreateBroadcast(ctx, management.Broadcast{
		ProjectID:  projectID,
		CampaignID: campaignID,
		ListID:     listID,
		ListName:   "Test List",
		ListType:   "static",
	})
	require.NoError(t, err)

	err = mgmtState.BroadcastsStore.TransitionPendingBroadcastToSending(ctx, broadcast.ID)
	require.NoError(t, err)

	registry := wasmProviders.NewRegistry(config.WASM{}, zap.NewNop())
	handler := BroadcastProcessHandler(logger, mgmtState, usrsState, registry, pub, ns)

	event := schemas.ProcessBroadcast{
		ProjectID:   projectID,
		BroadcastID: broadcast.ID,
	}
	err = pub.Publish(ctx, schemas.BroadcastsProcess(projectID, broadcast.ID), event)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamBroadcasts), ns.Consumer(ConsumerBroadcastsProcess))
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)

	batchConsumer, err := jet.Consumer(ctx, ns.Stream(StreamBroadcasts), ns.Consumer(ConsumerBroadcastsBatch))
	require.NoError(t, err)

	batchMsg, err := batchConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	var batchEvent schemas.ProcessBroadcastBatch
	err = json.Unmarshal(batchMsg.Data(), &batchEvent)
	require.NoError(t, err)

	assert.Equal(t, projectID, batchEvent.ProjectID)
	assert.Equal(t, broadcast.ID, batchEvent.BroadcastID)
}

// ----- BroadcastBatchHandler tests -----

func TestBroadcastBatchHandler_EmptyList(t *testing.T) {
	t.Parallel()

	mgmtState, usrsState, projectID, jet, ns := setupBroadcastsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))

	broadcastID, _, _ := seedBroadcast(t, ctx, mgmtState, usrsState, projectID)

	err = mgmtState.BroadcastsStore.TransitionPendingBroadcastToSending(ctx, broadcastID)
	require.NoError(t, err)

	handler := BroadcastBatchHandler(logger, mgmtState, usrsState, pub, ns)

	// The list has zero users, so the first batch should mark the broadcast as completed.
	batchEvent := schemas.ProcessBroadcastBatch{
		ProjectID:   projectID,
		BroadcastID: broadcastID,
		Offset:      0,
		BatchSize:   DefaultBroadcastBatchSize,
		Processed:   0,
	}
	err = pub.Publish(ctx, schemas.BroadcastsBatch(projectID, broadcastID), batchEvent)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamBroadcasts), ns.Consumer(ConsumerBroadcastsBatch))
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)

	// An empty list produces no messages for the campaign send handler to
	// complete the broadcast on, so the batch handler completes it directly
	// rather than leaving it in sending state forever.
	broadcast, err := mgmtState.BroadcastsStore.GetBroadcast(ctx, projectID, broadcastID)
	require.NoError(t, err)
	assert.Equal(t, management.BroadcastStateCompleted, broadcast.State)
	assert.Equal(t, 0, broadcast.Total)
}

func TestBroadcastBatchHandler_WithUsers(t *testing.T) {
	t.Parallel()

	mgmtState, usrsState, projectID, jet, ns := setupBroadcastsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))

	broadcastID, campaignID, listID := seedBroadcast(t, ctx, mgmtState, usrsState, projectID)

	// Add 3 users to the list.
	userIDs := make([]uuid.UUID, 3)
	for i := 0; i < 3; i++ {
		userIDs[i] = createTestUser(t, ctx, usrsState, projectID)
		err = usrsState.ListsStore.AddUserToList(ctx, listID, userIDs[i])
		require.NoError(t, err)
	}

	err = mgmtState.BroadcastsStore.TransitionPendingBroadcastToSending(ctx, broadcastID)
	require.NoError(t, err)

	handler := BroadcastBatchHandler(logger, mgmtState, usrsState, pub, ns)

	batchEvent := schemas.ProcessBroadcastBatch{
		ProjectID:   projectID,
		BroadcastID: broadcastID,
		Offset:      0,
		BatchSize:   DefaultBroadcastBatchSize,
		Processed:   0,
	}
	err = pub.Publish(ctx, schemas.BroadcastsBatch(projectID, broadcastID), batchEvent)
	require.NoError(t, err)

	batchConsumer, err := jet.Consumer(ctx, ns.Stream(StreamBroadcasts), ns.Consumer(ConsumerBroadcastsBatch))
	require.NoError(t, err)

	msg, err := batchConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)

	// With 3 users and batch size 1000, this is the last batch.
	// Broadcast stays in sending — completion happens when all
	// campaign messages have actually been delivered.
	broadcast, err := mgmtState.BroadcastsStore.GetBroadcast(ctx, projectID, broadcastID)
	require.NoError(t, err)
	assert.Equal(t, management.BroadcastStateSending, broadcast.State)
	assert.Equal(t, 3, broadcast.Total)

	// Verify that 3 SendCampaign messages were published.
	campaignConsumer, err := jet.Consumer(ctx, ns.Stream(StreamCampaigns), ns.Consumer(ConsumerCampaignsSend))
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		sendMsg, err := campaignConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
		require.NoError(t, err)

		var sendEvent schemas.SendCampaign
		err = json.Unmarshal(sendMsg.Data(), &sendEvent)
		require.NoError(t, err)

		assert.Equal(t, projectID, sendEvent.ProjectID)
		assert.Equal(t, campaignID, sendEvent.CampaignID)
		assert.NotNil(t, sendEvent.BroadcastID)
		assert.Equal(t, broadcastID, *sendEvent.BroadcastID)
	}
}

func TestBroadcastBatchHandler_ChainsNextBatch(t *testing.T) {
	t.Parallel()

	mgmtState, usrsState, projectID, jet, ns := setupBroadcastsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))

	broadcastID, _, listID := seedBroadcast(t, ctx, mgmtState, usrsState, projectID)

	// Use a small batch size (3) and add exactly 3 users so the batch is
	// "full" and the handler chains the next batch.
	batchSize := 3
	for i := 0; i < batchSize; i++ {
		userID := createTestUser(t, ctx, usrsState, projectID)
		err = usrsState.ListsStore.AddUserToList(ctx, listID, userID)
		require.NoError(t, err)
	}

	err = mgmtState.BroadcastsStore.TransitionPendingBroadcastToSending(ctx, broadcastID)
	require.NoError(t, err)

	handler := BroadcastBatchHandler(logger, mgmtState, usrsState, pub, ns)

	batchEvent := schemas.ProcessBroadcastBatch{
		ProjectID:   projectID,
		BroadcastID: broadcastID,
		Offset:      0,
		BatchSize:   batchSize,
		Processed:   0,
	}
	err = pub.Publish(ctx, schemas.BroadcastsBatch(projectID, broadcastID), batchEvent)
	require.NoError(t, err)

	batchConsumer, err := jet.Consumer(ctx, ns.Stream(StreamBroadcasts), ns.Consumer(ConsumerBroadcastsBatch))
	require.NoError(t, err)

	msg, err := batchConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)

	// The handler should have chained another batch message because the
	// batch was exactly full (len(userIDs) == batchSize).
	nextMsg, err := batchConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	var nextBatch schemas.ProcessBroadcastBatch
	err = json.Unmarshal(nextMsg.Data(), &nextBatch)
	require.NoError(t, err)

	assert.Equal(t, projectID, nextBatch.ProjectID)
	assert.Equal(t, broadcastID, nextBatch.BroadcastID)
	assert.Equal(t, batchSize, nextBatch.Offset)
	assert.Equal(t, batchSize, nextBatch.BatchSize)
	assert.Equal(t, batchSize, nextBatch.Processed)

	// Processing the next batch (no more users) should complete the broadcast.
	err = handler(ctx, nextMsg)
	require.NoError(t, err)

	err = nextMsg.Ack()
	require.NoError(t, err)

	broadcast, err := mgmtState.BroadcastsStore.GetBroadcast(ctx, projectID, broadcastID)
	require.NoError(t, err)
	assert.Equal(t, management.BroadcastStateSending, broadcast.State)
	assert.Equal(t, batchSize, broadcast.Total)
}

func TestBroadcastBatchHandler_CancelledDuringProcessing(t *testing.T) {
	t.Parallel()

	mgmtState, usrsState, projectID, jet, ns := setupBroadcastsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))

	broadcastID, _, listID := seedBroadcast(t, ctx, mgmtState, usrsState, projectID)

	// Add users to the list.
	for i := 0; i < 5; i++ {
		userID := createTestUser(t, ctx, usrsState, projectID)
		err = usrsState.ListsStore.AddUserToList(ctx, listID, userID)
		require.NoError(t, err)
	}

	err = mgmtState.BroadcastsStore.TransitionPendingBroadcastToSending(ctx, broadcastID)
	require.NoError(t, err)

	// Now mark the broadcast as cancelled (simulating a user cancel mid-flight).
	err = mgmtState.BroadcastsStore.UpdateBroadcastState(ctx, projectID, broadcastID, management.BroadcastStateCancelled, 0, nil)
	require.NoError(t, err)

	handler := BroadcastBatchHandler(logger, mgmtState, usrsState, pub, ns)

	batchEvent := schemas.ProcessBroadcastBatch{
		ProjectID:   projectID,
		BroadcastID: broadcastID,
		Offset:      0,
		BatchSize:   DefaultBroadcastBatchSize,
		Processed:   0,
	}
	err = pub.Publish(ctx, schemas.BroadcastsBatch(projectID, broadcastID), batchEvent)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamBroadcasts), ns.Consumer(ConsumerBroadcastsBatch))
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.Error(t, err)
	assert.True(t, IsPermanent(err), "error should be permanent when broadcast is cancelled")

	// Broadcast state should still be cancelled (no side effects).
	broadcast, err := mgmtState.BroadcastsStore.GetBroadcast(ctx, projectID, broadcastID)
	require.NoError(t, err)
	assert.Equal(t, management.BroadcastStateCancelled, broadcast.State)
}

func TestBroadcastBatchHandler_InvalidMessage(t *testing.T) {
	t.Parallel()

	mgmtState, usrsState, projectID, jet, ns := setupBroadcastsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))
	handler := BroadcastBatchHandler(logger, mgmtState, usrsState, pub, ns)

	// Publish an invalid JSON message.
	_ = projectID
	broadcastID := uuid.New()
	err = pub.Publish(ctx, schemas.BroadcastsBatch(projectID, broadcastID), "not-valid-json{{{")
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamBroadcasts), ns.Consumer(ConsumerBroadcastsBatch))
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.Error(t, err)
	assert.True(t, IsPermanent(err), "malformed JSON should cause a permanent error")
}

func TestBroadcastProcessHandler_InvalidMessage(t *testing.T) {
	t.Parallel()

	mgmtState, usrsState, projectID, jet, ns := setupBroadcastsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))
	registry := wasmProviders.NewRegistry(config.WASM{}, zap.NewNop())
	handler := BroadcastProcessHandler(logger, mgmtState, usrsState, registry, pub, ns)

	broadcastID := uuid.New()
	err = pub.Publish(ctx, schemas.BroadcastsProcess(projectID, broadcastID), "not-valid-json{{{")
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamBroadcasts), ns.Consumer(ConsumerBroadcastsProcess))
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.Error(t, err)
	assert.True(t, IsPermanent(err), "malformed JSON should cause a permanent error")
}

func TestBroadcastProcessHandler_BroadcastNotFound(t *testing.T) {
	t.Parallel()

	mgmtState, usrsState, projectID, jet, ns := setupBroadcastsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))
	registry := wasmProviders.NewRegistry(config.WASM{}, zap.NewNop())
	handler := BroadcastProcessHandler(logger, mgmtState, usrsState, registry, pub, ns)

	nonExistentID := uuid.New()
	event := schemas.ProcessBroadcast{
		ProjectID:   projectID,
		BroadcastID: nonExistentID,
	}
	err = pub.Publish(ctx, schemas.BroadcastsProcess(projectID, nonExistentID), event)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamBroadcasts), ns.Consumer(ConsumerBroadcastsProcess))
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.Error(t, err)
	assert.True(t, IsPermanent(err), "non-existent broadcast should cause a permanent error")
}

func TestBroadcastBatchHandler_BroadcastNotFound(t *testing.T) {
	t.Parallel()

	mgmtState, usrsState, projectID, jet, ns := setupBroadcastsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))
	handler := BroadcastBatchHandler(logger, mgmtState, usrsState, pub, ns)

	nonExistentID := uuid.New()
	batchEvent := schemas.ProcessBroadcastBatch{
		ProjectID:   projectID,
		BroadcastID: nonExistentID,
		Offset:      0,
		BatchSize:   DefaultBroadcastBatchSize,
		Processed:   0,
	}
	err = pub.Publish(ctx, schemas.BroadcastsBatch(projectID, nonExistentID), batchEvent)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamBroadcasts), ns.Consumer(ConsumerBroadcastsBatch))
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.Error(t, err)
	assert.True(t, IsPermanent(err), "non-existent broadcast should cause a permanent error")
}

// TestBroadcastEndToEnd exercises the full process → batch → completion
// flow in a single test.
func TestBroadcastEndToEnd(t *testing.T) {
	t.Parallel()

	mgmtState, usrsState, projectID, jet, ns := setupBroadcastsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))

	broadcastID, campaignID, listID := seedBroadcast(t, ctx, mgmtState, usrsState, projectID)

	// Add 5 users to the list.
	numUsers := 5
	for i := 0; i < numUsers; i++ {
		userID := createTestUser(t, ctx, usrsState, projectID)
		err = usrsState.ListsStore.AddUserToList(ctx, listID, userID)
		require.NoError(t, err)
	}

	// Transition to sending.
	err = mgmtState.BroadcastsStore.TransitionPendingBroadcastToSending(ctx, broadcastID)
	require.NoError(t, err)

	// Step 1: Invoke process handler.
	registry := wasmProviders.NewRegistry(config.WASM{}, zap.NewNop())
	processHandler := BroadcastProcessHandler(logger, mgmtState, usrsState, registry, pub, ns)

	processEvent := schemas.ProcessBroadcast{
		ProjectID:   projectID,
		BroadcastID: broadcastID,
	}
	err = pub.Publish(ctx, schemas.BroadcastsProcess(projectID, broadcastID), processEvent)
	require.NoError(t, err)

	processConsumer, err := jet.Consumer(ctx, ns.Stream(StreamBroadcasts), ns.Consumer(ConsumerBroadcastsProcess))
	require.NoError(t, err)

	processMsg, err := processConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = processHandler(ctx, processMsg)
	require.NoError(t, err)
	err = processMsg.Ack()
	require.NoError(t, err)

	// Step 2: Consume and invoke batch handler (should complete in one batch).
	batchHandler := BroadcastBatchHandler(logger, mgmtState, usrsState, pub, ns)

	batchConsumer, err := jet.Consumer(ctx, ns.Stream(StreamBroadcasts), ns.Consumer(ConsumerBroadcastsBatch))
	require.NoError(t, err)

	batchMsg, err := batchConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = batchHandler(ctx, batchMsg)
	require.NoError(t, err)
	err = batchMsg.Ack()
	require.NoError(t, err)

	// Step 3: Verify state — broadcast stays in "sending" until the
	// campaign send handler delivers all messages and marks it completed.
	broadcast, err := mgmtState.BroadcastsStore.GetBroadcast(ctx, projectID, broadcastID)
	require.NoError(t, err)
	assert.Equal(t, management.BroadcastStateSending, broadcast.State)
	assert.Equal(t, numUsers, broadcast.Total)

	// Verify SendCampaign messages.
	sendConsumer, err := jet.Consumer(ctx, ns.Stream(StreamCampaigns), ns.Consumer(ConsumerCampaignsSend))
	require.NoError(t, err)

	for i := 0; i < numUsers; i++ {
		sendMsg, err := sendConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
		require.NoError(t, err)

		var sendEvent schemas.SendCampaign
		err = json.Unmarshal(sendMsg.Data(), &sendEvent)
		require.NoError(t, err)

		assert.Equal(t, projectID, sendEvent.ProjectID)
		assert.Equal(t, campaignID, sendEvent.CampaignID)
		assert.NotNil(t, sendEvent.BroadcastID)
		assert.Equal(t, broadcastID, *sendEvent.BroadcastID)
	}
}
