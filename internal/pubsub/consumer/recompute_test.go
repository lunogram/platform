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
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/lunogram/platform/internal/store/users"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func setupRecomputeTest(t *testing.T) (*users.State, uuid.UUID, jetstream.JetStream) {
	t.Helper()

	ctx := graceful.NewContext(t.Context())

	natsURL := container.RunNATS(t)
	mgmt, usrs, _ := teststore.RunPostgreSQL(t)

	cfg := config.Node{
		Nats: config.Nats{
			URL: natsURL,
		},
	}

	jet, err := pubsub.New(ctx, cfg)
	require.NoError(t, err)

	mgmtState := management.NewState(mgmt)
	usersState := users.NewState(usrs)

	orgID, err := mgmtState.OrganizationsStore.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projectID, err := mgmtState.ProjectsStore.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	return usersState, projectID, jet
}

func TestListsRecompute(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	listID := uuid.New()

	subject := schemas.ListsRecompute(projectID, listID)
	expected := schemas.Subject("lists.recompute." + projectID.String() + "." + listID.String())

	assert.Equal(t, expected, subject)
}

func TestRecomputeListHandlerSuccess(t *testing.T) {
	t.Parallel()

	st, projectID, jet := setupRecomputeTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	email := "test@example.com"
	userID, err := st.UsersStore.UpsertUser(ctx, projectID, users.UpsertUserParams{
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

	ruleID, err := st.RulesStore.CreateRule(ctx, users.Rule{
		ProjectID:       projectID,
		Rule:            store.JSONB[rules.RuleSet]{Data: ruleset},
		DependsOnUsers:  true,
		DependsOnEvents: false,
		Version:         1,
	})
	require.NoError(t, err)

	listID, err := st.ListsStore.CreateList(ctx, users.List{
		ProjectID: projectID,
		Name:      "Test List",
		Type:      "static",
		RuleID:    &ruleID,
	})
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)
	handler := RecomputeListHandler(logger, st, pub)

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

	usrs, total, err := st.ListsStore.SelectListUsers(ctx, projectID, listID, store.Pagination{Limit: 10, Offset: 0})
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Len(t, usrs, 1)
	assert.Equal(t, userID, usrs[0].ID)
}

func TestRecomputeListHandlerNoRule(t *testing.T) {
	t.Parallel()

	st, projectID, jet := setupRecomputeTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	listID, err := st.ListsStore.CreateList(ctx, users.List{
		ProjectID: projectID,
		Name:      "Test List Without Rule",
		Type:      "static",
		RuleID:    nil,
	})
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)
	handler := RecomputeListHandler(logger, st, pub)

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

func TestRecomputeListHandlerWithUserAddedEvent(t *testing.T) {
	t.Parallel()

	st, projectID, jet := setupRecomputeTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	email := "test@example.com"
	userID, err := st.UsersStore.UpsertUser(ctx, projectID, users.UpsertUserParams{
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

	ruleID, err := st.RulesStore.CreateRule(ctx, users.Rule{
		ProjectID:       projectID,
		Rule:            store.JSONB[rules.RuleSet]{Data: ruleset},
		DependsOnUsers:  true,
		DependsOnEvents: false,
		Version:         1,
	})
	require.NoError(t, err)

	listID, err := st.ListsStore.CreateList(ctx, users.List{
		ProjectID: projectID,
		Name:      "Test List",
		Type:      "static",
		RuleID:    &ruleID,
	})
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)
	handler := RecomputeListHandler(logger, st, pub)

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

func TestPublishListRecomputeEventsInserted(t *testing.T) {
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

	recomputed := []users.Recomputed{
		{
			UserID: userID,
			Action: users.RecomputeActionInserted,
		},
	}

	err = PublishListRecomputeEvents(ctx, logger, pub, projectID, listID, recomputed)
	require.NoError(t, err)
}

func TestPublishListRecomputeEventsDeleted(t *testing.T) {
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

	recomputed := []users.Recomputed{
		{
			UserID: userID,
			Action: users.RecomputeActionDeleted,
		},
	}

	err = PublishListRecomputeEvents(ctx, logger, pub, projectID, listID, recomputed)
	require.NoError(t, err)
}

func TestPublishListRecomputeEventsMixed(t *testing.T) {
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

	recomputed := []users.Recomputed{
		{
			UserID: uuid.New(),
			Action: users.RecomputeActionInserted,
		},
		{
			UserID: uuid.New(),
			Action: users.RecomputeActionDeleted,
		},
		{
			UserID: uuid.New(),
			Action: users.RecomputeActionInserted,
		},
	}

	err = PublishListRecomputeEvents(ctx, logger, pub, projectID, listID, recomputed)
	require.NoError(t, err)
}
