package consumer

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/container"
	"github.com/lunogram/platform/services/nexus/internal/pubsub"
	"github.com/lunogram/platform/services/nexus/internal/pubsub/schemas"
	"github.com/lunogram/platform/services/nexus/internal/rules"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func setupRecomputeTest(t *testing.T) (*sqlx.DB, uuid.UUID, jetstream.JetStream) {
	t.Helper()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	natsURL := container.RunNATS(t)
	postgresURI := container.RunPostgreSQL(t)

	config := config.Node{
		Store: store.Config{
			URI: postgresURI,
		},
		Nats: config.Nats{
			URL: natsURL,
		},
	}

	err := store.Migrate(config.Store)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, config.Store)
	require.NoError(t, err)

	jet, err := pubsub.New(ctx, config)
	require.NoError(t, err)

	st := store.NewState(db)

	orgID, err := st.OrganizationsStore.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projectID, err := st.ProjectsStore.CreateProject(ctx, store.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	return db, projectID, jet
}

func TestListsRecompute(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	listID := uuid.New()

	subject := schemas.ListsRecompute(projectID, listID)
	expected := schemas.Subject("lists.recompute." + projectID.String() + "." + listID.String())

	assert.Equal(t, expected, subject)
}

func TestRecomputeListHandler_Success(t *testing.T) {
	t.Parallel()

	db, projectID, jet := setupRecomputeTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	st := store.NewState(db)

	email := "test@example.com"
	userID, err := st.UsersStore.UpsertUser(ctx, projectID, store.UpsertUserParams{
		Email: &email,
		Data: map[string]any{
			"age": 25,
		},
	})
	require.NoError(t, err)

	ruleset := rules.RuleSet{
		Rule: rules.Rule{
			UUID:     uuid.New(),
			Path:     "user.data.age",
			Group:    rules.RuleGroupUser,
			Type:     rules.RuleTypeNumber,
			Operator: rules.OperatorGreaterEqual,
			Value:    float64(18),
		},
	}

	ruleID, err := st.RulesStore.CreateRule(ctx, store.Rule{
		ProjectID:       projectID,
		Rule:            store.JSONB[rules.RuleSet]{Data: ruleset},
		DependsOnUsers:  true,
		DependsOnEvents: false,
		Version:         1,
	})
	require.NoError(t, err)

	listID, err := st.ListsStore.CreateList(ctx, store.List{
		ProjectID: projectID,
		Name:      "Test List",
		Type:      "static",
		RuleID:    &ruleID,
	})
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)
	handler := RecomputeListHandler(logger, db, pub)

	recompute := RecomputeList{
		ID:        listID,
		ProjectID: projectID,
	}

	err = pub.Publish(ctx, schemas.ListsRecompute(projectID, listID), recompute)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamLists, ConsumerListsRecompute)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)

	users, total, err := st.ListsStore.SelectListUsers(ctx, projectID, listID, store.Pagination{Limit: 10, Offset: 0})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, users, 1)
	assert.Equal(t, userID, users[0].ID)
}

func TestRecomputeListHandler_NoRule(t *testing.T) {
	t.Parallel()

	db, projectID, jet := setupRecomputeTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	st := store.NewState(db)

	listID, err := st.ListsStore.CreateList(ctx, store.List{
		ProjectID: projectID,
		Name:      "Test List Without Rule",
		Type:      "static",
		RuleID:    nil,
	})
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)
	handler := RecomputeListHandler(logger, db, pub)

	recompute := RecomputeList{
		ID:        listID,
		ProjectID: projectID,
	}

	err = pub.Publish(ctx, schemas.ListsRecompute(projectID, listID), recompute)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamLists, ConsumerListsRecompute)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)
}

func TestRecomputeListHandler_WithUserAddedEvent(t *testing.T) {
	t.Parallel()

	db, projectID, jet := setupRecomputeTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	st := store.NewState(db)

	email := "test@example.com"
	userID, err := st.UsersStore.UpsertUser(ctx, projectID, store.UpsertUserParams{
		Email: &email,
		Data: map[string]any{
			"age": 30,
		},
	})
	require.NoError(t, err)

	ruleset := rules.RuleSet{
		Rule: rules.Rule{
			UUID:     uuid.New(),
			Path:     "user.data.age",
			Group:    rules.RuleGroupUser,
			Type:     rules.RuleTypeNumber,
			Operator: rules.OperatorGreaterEqual,
			Value:    float64(18),
		},
	}

	ruleID, err := st.RulesStore.CreateRule(ctx, store.Rule{
		ProjectID:       projectID,
		Rule:            store.JSONB[rules.RuleSet]{Data: ruleset},
		DependsOnUsers:  true,
		DependsOnEvents: false,
		Version:         1,
	})
	require.NoError(t, err)

	listID, err := st.ListsStore.CreateList(ctx, store.List{
		ProjectID: projectID,
		Name:      "Test List",
		Type:      "static",
		RuleID:    &ruleID,
	})
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)
	handler := RecomputeListHandler(logger, db, pub)

	recompute := RecomputeList{
		ID:        listID,
		ProjectID: projectID,
	}

	err = pub.Publish(ctx, schemas.ListsRecompute(projectID, listID), recompute)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamLists, ConsumerListsRecompute)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)

	eventConsumer, err := jet.Consumer(ctx, StreamEvents, ConsumerEventsProcess)
	require.NoError(t, err)

	eventMsg, err := eventConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	var receivedEvent schemas.Event
	err = json.Unmarshal(eventMsg.Data(), &receivedEvent)
	require.NoError(t, err)
	assert.Equal(t, schemas.EventListUserAdded, receivedEvent.Name)
	assert.Equal(t, userID, receivedEvent.UserID)
	assert.Equal(t, projectID, receivedEvent.ProjectID)
}

func TestPublishListRecomputeEvents_Inserted(t *testing.T) {
	t.Parallel()

	jet := setupBootstrapTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	_, err := jet.CreateStream(ctx, jetstream.StreamConfig{
		Name:     StreamEvents,
		Subjects: []string{"events.>"},
	})
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)
	projectID := uuid.New()
	listID := uuid.New()
	userID := uuid.New()

	recomputed := []store.Recomputed{
		{
			UserID: userID,
			Action: store.RecomputeActionInserted,
		},
	}

	err = PublishListRecomputeEvents(ctx, logger, pub, projectID, listID, recomputed)
	require.NoError(t, err)
}

func TestPublishListRecomputeEvents_Deleted(t *testing.T) {
	t.Parallel()

	jet := setupBootstrapTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	_, err := jet.CreateStream(ctx, jetstream.StreamConfig{
		Name:     StreamEvents,
		Subjects: []string{"events.>"},
	})
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)
	projectID := uuid.New()
	listID := uuid.New()
	userID := uuid.New()

	recomputed := []store.Recomputed{
		{
			UserID: userID,
			Action: store.RecomputeActionDeleted,
		},
	}

	err = PublishListRecomputeEvents(ctx, logger, pub, projectID, listID, recomputed)
	require.NoError(t, err)
}

func TestPublishListRecomputeEvents_Mixed(t *testing.T) {
	t.Parallel()

	jet := setupBootstrapTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	_, err := jet.CreateStream(ctx, jetstream.StreamConfig{
		Name:     StreamEvents,
		Subjects: []string{"events.>"},
	})
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)
	projectID := uuid.New()
	listID := uuid.New()

	recomputed := []store.Recomputed{
		{
			UserID: uuid.New(),
			Action: store.RecomputeActionInserted,
		},
		{
			UserID: uuid.New(),
			Action: store.RecomputeActionDeleted,
		},
		{
			UserID: uuid.New(),
			Action: store.RecomputeActionInserted,
		},
	}

	err = PublishListRecomputeEvents(ctx, logger, pub, projectID, listID, recomputed)
	require.NoError(t, err)
}
