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
	"github.com/lunogram/platform/internal/store/subjects"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func setupUsersTest(t *testing.T) (*subjects.State, uuid.UUID, jetstream.JetStream, Namespace) {
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

	orgID, err := mgmtState.OrganizationsStore.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projectID, err := mgmtState.ProjectsStore.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	usersState := subjects.NewState(usrs, zap.NewNop())
	ns := testNamespace(t)

	return usersState, projectID, jet, ns
}

func TestUsersProcess(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	subject := schemas.UsersProcess(projectID)
	expected := schemas.Subject("users.process." + projectID.String())

	assert.Equal(t, expected, subject)
}

func TestUsersSchema(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	subject := schemas.UsersSchema(projectID)
	expected := schemas.Subject("users.schema." + projectID.String())

	assert.Equal(t, expected, subject)
}

func TestUserEvent(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	userID := uuid.New()
	email := "test@example.com"
	phone := "+1234567890"
	timezone := "America/New_York"
	locale := "en"

	user := schemas.User{
		ID:        userID,
		ProjectID: projectID,
		Identifiers: []schemas.ExternalID{
			{Source: "default", ExternalID: "ext456"},
			{Source: "anonymous", ExternalID: "anon123"},
		},
		Email:    &email,
		Phone:    &phone,
		Timezone: &timezone,
		Locale:   &locale,
		Data: map[string]any{
			"key": "value",
		},
		Version: 5,
	}

	event := user.UserEvent("test_event")

	assert.Equal(t, "test_event", event.Name)
	assert.Equal(t, projectID, event.ProjectID)
	require.Len(t, event.Identifiers, 2)
	assert.Equal(t, "default", event.Identifiers[0].Source)
	assert.Equal(t, "ext456", event.Identifiers[0].ExternalID)
	assert.Equal(t, "anonymous", event.Identifiers[1].Source)
	assert.Equal(t, "anon123", event.Identifiers[1].ExternalID)
	require.NotNil(t, event.Data)

	assert.Equal(t, userID, event.Data["id"])
	assert.Equal(t, &email, event.Data["email"])
	assert.Equal(t, &phone, event.Data["phone"])
	assert.Equal(t, &timezone, event.Data["timezone"])
	assert.Equal(t, &locale, event.Data["locale"])
	assert.Equal(t, int32(5), event.Data["version"])
}

func TestUsersHandlerSuccess(t *testing.T) {
	t.Parallel()

	usersState, projectID, jet, ns := setupUsersTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))
	handler := UsersHandler(logger, usersState, pub)

	email := "test@example.com"
	user := schemas.User{
		ID:        uuid.New(),
		ProjectID: projectID,
		Email:     &email,
		Data: map[string]any{
			"age": 25,
		},
		Version: 0,
	}

	err = pub.Publish(ctx, schemas.UsersProcess(projectID), user)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamUsers), ns.Consumer(ConsumerUsersProcess))
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)
}

func TestUsersHandlerPublishesUserCreatedEvent(t *testing.T) {
	t.Parallel()

	usersState, projectID, jet, ns := setupUsersTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))
	handler := UsersHandler(logger, usersState, pub)

	email := "new@example.com"
	user := schemas.User{
		ID:        uuid.New(),
		ProjectID: projectID,
		Email:     &email,
		Data: map[string]any{
			"name": "Test User",
		},
		Version: 0,
	}

	err = pub.Publish(ctx, schemas.UsersProcess(projectID), user)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamUsers), ns.Consumer(ConsumerUsersProcess))
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	eventConsumer, err := jet.Consumer(ctx, ns.Stream(StreamUserEvents), ns.Consumer(ConsumerUserEventsProcess))
	require.NoError(t, err)

	eventMsg, err := eventConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	var receivedEvent schemas.UserEvent
	err = json.Unmarshal(eventMsg.Data(), &receivedEvent)
	require.NoError(t, err)
	assert.Equal(t, schemas.EventUserCreated, receivedEvent.Name)
	assert.Equal(t, projectID, receivedEvent.ProjectID)
}

func TestUsersHandlerPublishesUserUpdatedEvent(t *testing.T) {
	t.Parallel()

	usersState, projectID, jet, ns := setupUsersTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))
	handler := UsersHandler(logger, usersState, pub)

	email := "existing@example.com"
	user := schemas.User{
		ID:        uuid.New(),
		ProjectID: projectID,
		Email:     &email,
		Data: map[string]any{
			"name": "Updated User",
		},
		Version: 5,
	}

	err = pub.Publish(ctx, schemas.UsersProcess(projectID), user)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamUsers), ns.Consumer(ConsumerUsersProcess))
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	eventConsumer, err := jet.Consumer(ctx, ns.Stream(StreamUserEvents), ns.Consumer(ConsumerUserEventsProcess))
	require.NoError(t, err)

	eventMsg, err := eventConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	var receivedEvent schemas.UserEvent
	err = json.Unmarshal(eventMsg.Data(), &receivedEvent)
	require.NoError(t, err)
	assert.Equal(t, schemas.EventUserUpdated, receivedEvent.Name)
	assert.Equal(t, projectID, receivedEvent.ProjectID)
}

func TestUsersHandlerWithListDependencies(t *testing.T) {
	t.Parallel()

	usersState, projectID, jet, ns := setupUsersTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet, ns)
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

	ruleID, err := usersState.RulesStore.CreateRule(ctx, subjects.Rule{
		ProjectID:       projectID,
		Rule:            store.JSONB[rules.RuleSet]{Data: ruleset},
		DependsOnUsers:  true,
		DependsOnEvents: false,
		Version:         1,
	})
	require.NoError(t, err)

	listID, err := usersState.ListsStore.CreateList(ctx, subjects.List{
		ProjectID: projectID,
		Name:      "Test List",
		Type:      "static",
	})
	require.NoError(t, err)

	versionID, err := usersState.ListsStore.CreateVersion(ctx, listID, &ruleID)
	require.NoError(t, err)

	err = usersState.ListsStore.PublishVersion(ctx, listID, versionID)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))
	handler := UsersHandler(logger, usersState, pub)

	email := "test@example.com"
	user := schemas.User{
		ID:        uuid.New(),
		ProjectID: projectID,
		Email:     &email,
		Data: map[string]any{
			"age": 25,
		},
		Version: 0,
	}

	err = pub.Publish(ctx, schemas.UsersProcess(projectID), user)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamUsers), ns.Consumer(ConsumerUsersProcess))
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	recomputeConsumer, err := jet.Consumer(ctx, ns.Stream(StreamLists), ns.Consumer(ConsumerListsRecompute))
	require.NoError(t, err)

	recomputeMsg, err := recomputeConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	var recompute RecomputeList
	err = json.Unmarshal(recomputeMsg.Data(), &recompute)
	require.NoError(t, err)
	assert.Equal(t, projectID, recompute.ProjectID)
}

func TestUsersHandlerWithUserData(t *testing.T) {
	t.Parallel()

	usersState, projectID, jet, ns := setupUsersTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))
	handler := UsersHandler(logger, usersState, pub)

	email := "test@example.com"
	user := schemas.User{
		ID:        uuid.New(),
		ProjectID: projectID,
		Email:     &email,
		Data: map[string]any{
			"age":  30,
			"name": "John Doe",
		},
		Version: 0,
	}

	err = pub.Publish(ctx, schemas.UsersProcess(projectID), user)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamUsers), ns.Consumer(ConsumerUsersProcess))
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	schemaConsumer, err := jet.Consumer(ctx, ns.Stream(StreamUsers), ns.Consumer(ConsumerUsersSchema))
	require.NoError(t, err)

	schemaMsg, err := schemaConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	var receivedUser schemas.User
	err = json.Unmarshal(schemaMsg.Data(), &receivedUser)
	require.NoError(t, err)
	assert.Equal(t, user.ID, receivedUser.ID)
	assert.NotNil(t, receivedUser.Data)
}

func TestUsersHandlerWithoutData(t *testing.T) {
	t.Parallel()

	usersState, projectID, jet, ns := setupUsersTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))
	handler := UsersHandler(logger, usersState, pub)

	email := "test@example.com"
	user := schemas.User{
		ID:        uuid.New(),
		ProjectID: projectID,
		Email:     &email,
		Data:      nil,
		Version:   0,
	}

	err = pub.Publish(ctx, schemas.UsersProcess(projectID), user)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamUsers), ns.Consumer(ConsumerUsersProcess))
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	schemaConsumer, err := jet.Consumer(ctx, ns.Stream(StreamUsers), ns.Consumer(ConsumerUsersSchema))
	require.NoError(t, err)

	_, err = schemaConsumer.Next(jetstream.FetchMaxWait(1 * time.Second))
	assert.Error(t, err)
}

func TestPublishUserRecomputeListsSuccess(t *testing.T) {
	t.Parallel()

	usersState, projectID, jet, ns := setupUsersTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	ruleset := rules.RuleSet{
		Rule: rules.Rule{
			UUID:     uuid.New(),
			Path:     "user.data.age",
			Group:    rules.RuleGroupUser,
			Type:     rules.RuleTypeNumber,
			Operator: rules.OperatorGreaterEqual,
			Value:    float64(21),
		},
	}

	ruleID, err := usersState.RulesStore.CreateRule(ctx, subjects.Rule{
		ProjectID:       projectID,
		Rule:            store.JSONB[rules.RuleSet]{Data: ruleset},
		DependsOnUsers:  true,
		DependsOnEvents: false,
		Version:         1,
	})
	require.NoError(t, err)

	listID, err := usersState.ListsStore.CreateList(ctx, subjects.List{
		ProjectID: projectID,
		Name:      "Adult List",
		Type:      "static",
	})
	require.NoError(t, err)

	versionID, err := usersState.ListsStore.CreateVersion(ctx, listID, &ruleID)
	require.NoError(t, err)

	err = usersState.ListsStore.PublishVersion(ctx, listID, versionID)
	require.NoError(t, err)

	_, err = jet.CreateStream(ctx, jetstream.StreamConfig{
		Name:     ns.Stream(StreamLists),
		Subjects: []string{ns.Subject("lists.recompute.>")},
	})
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))

	email := "test@example.com"
	user := schemas.User{
		ID:        uuid.New(),
		ProjectID: projectID,
		Email:     &email,
		Data: map[string]any{
			"age": 25,
		},
		Version: 0,
	}

	err = PublishUserRecomputeLists(ctx, logger, usersState, pub, user)
	require.NoError(t, err)

	stream, err := jet.Stream(ctx, ns.Stream(StreamLists))
	require.NoError(t, err)

	info, err := stream.Info(ctx)
	require.NoError(t, err)
	assert.Greater(t, info.State.Msgs, uint64(0))

	result, err := usersState.ListsStore.SelectListUsersDependency(ctx, projectID)
	require.NoError(t, err)
	assert.Contains(t, result, listID)
}

func TestPublishUserRecomputeListsNoLists(t *testing.T) {
	t.Parallel()

	usersState, projectID, jet, ns := setupUsersTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	_, err := jet.CreateStream(ctx, jetstream.StreamConfig{
		Name:     ns.Stream(StreamLists),
		Subjects: []string{ns.Subject("lists.recompute.>")},
	})
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))

	email := "test@example.com"
	user := schemas.User{
		ID:        uuid.New(),
		ProjectID: projectID,
		Email:     &email,
		Data: map[string]any{
			"age": 25,
		},
		Version: 0,
	}

	err = PublishUserRecomputeLists(ctx, logger, usersState, pub, user)
	require.NoError(t, err)
}

func TestPublishUserEventsUserCreated(t *testing.T) {
	t.Parallel()

	jet := setupBootstrapTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	ns := testNamespace(t)

	_, err := jet.CreateStream(ctx, jetstream.StreamConfig{
		Name:     ns.Stream(StreamUserEvents),
		Subjects: []string{ns.Subject("users.events.>")},
	})
	require.NoError(t, err)

	_, err = jet.CreateStream(ctx, jetstream.StreamConfig{
		Name:     ns.Stream(StreamScheduled),
		Subjects: []string{ns.Subject("scheduled.process.>"), ns.Subject("scheduled.schema.>"), ns.Subject("scheduled.backfill.>")},
	})
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))
	projectID := uuid.New()

	email := "new@example.com"
	user := schemas.User{
		ID:        uuid.New(),
		ProjectID: projectID,
		Email:     &email,
		Data: map[string]any{
			"name": "New User",
		},
		Version: 0,
	}

	err = PublishUserEvents(ctx, logger, pub, user)
	require.NoError(t, err)
}

func TestPublishUserEventsUserUpdated(t *testing.T) {
	t.Parallel()

	jet := setupBootstrapTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	ns := testNamespace(t)

	_, err := jet.CreateStream(ctx, jetstream.StreamConfig{
		Name:     ns.Stream(StreamUserEvents),
		Subjects: []string{ns.Subject("users.events.>")},
	})
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, string(ns))
	projectID := uuid.New()

	email := "existing@example.com"
	user := schemas.User{
		ID:        uuid.New(),
		ProjectID: projectID,
		Email:     &email,
		Data: map[string]any{
			"name": "Updated User",
		},
		Version: 3,
	}

	err = PublishUserEvents(ctx, logger, pub, user)
	require.NoError(t, err)
}

func TestUserSchemasHandlerSuccess(t *testing.T) {
	t.Parallel()

	usersState, projectID, jet, ns := setupUsersTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	handler := UserSchemasHandler(logger, usersState)

	email := "test@example.com"
	user := schemas.User{
		ID:        uuid.New(),
		ProjectID: projectID,
		Email:     &email,
		Data: map[string]any{
			"profile": map[string]any{
				"name":    "John Doe",
				"age":     30,
				"country": "USA",
			},
			"preferences": map[string]any{
				"newsletter": true,
			},
		},
		Version: 0,
	}

	err = pubsub.NewPublisher(jet, string(ns)).Publish(ctx, schemas.UsersSchema(projectID), user)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamUsers), ns.Consumer(ConsumerUsersSchema))
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)
}

func TestUserSchemasHandlerComplexData(t *testing.T) {
	t.Parallel()

	usersState, projectID, jet, ns := setupUsersTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	handler := UserSchemasHandler(logger, usersState)

	email := "test@example.com"
	user := schemas.User{
		ID:        uuid.New(),
		ProjectID: projectID,
		Email:     &email,
		Data: map[string]any{
			"profile": map[string]any{
				"name": "Jane Doe",
				"contact": map[string]any{
					"email": "jane@example.com",
					"phone": "+1234567890",
				},
				"address": map[string]any{
					"street": "123 Main St",
					"city":   "New York",
					"zip":    "10001",
				},
			},
			"purchases": []any{
				map[string]any{
					"id":     "order_1",
					"amount": 99.99,
				},
			},
		},
		Version: 0,
	}

	err = pubsub.NewPublisher(jet, string(ns)).Publish(ctx, schemas.UsersSchema(projectID), user)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamUsers), ns.Consumer(ConsumerUsersSchema))
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)
}
