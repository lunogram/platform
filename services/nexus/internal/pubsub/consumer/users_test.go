package consumer

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/pkg/container"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/pubsub"
	"github.com/lunogram/platform/services/nexus/internal/pubsub/schemas"
	"github.com/lunogram/platform/services/nexus/internal/rules"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func setupUsersTest(t *testing.T) (*sqlx.DB, uuid.UUID, jetstream.JetStream) {
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

func TestUsersProjectSubject(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	subject := schemas.UsersProjectSubject(projectID)
	expected := schemas.Subject("users.projects." + projectID.String())

	assert.Equal(t, expected, subject)
}

func TestUsersSchemaSubject(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	subject := schemas.UsersSchemaSubject(projectID)
	expected := schemas.Subject("users.schemas." + projectID.String())

	assert.Equal(t, expected, subject)
}

func TestUser_Event(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	userID := uuid.New()
	anonID := "anon123"
	externalID := "ext456"
	email := "test@example.com"
	phone := "+1234567890"
	timezone := "America/New_York"
	locale := "en-US"

	user := schemas.User{
		ID:          userID,
		ProjectID:   projectID,
		AnonymousID: &anonID,
		ExternalID:  &externalID,
		Email:       &email,
		Phone:       &phone,
		Timezone:    &timezone,
		Locale:      &locale,
		Data: map[string]any{
			"key": "value",
		},
		Version: 5,
	}

	event := user.Event("test_event")

	assert.Equal(t, "test_event", event.Name)
	assert.Equal(t, projectID, event.ProjectID)
	assert.Equal(t, &anonID, event.AnonymousId)
	assert.Equal(t, &externalID, event.ExternalId)
	require.NotNil(t, event.Data)

	assert.Equal(t, userID, event.Data["id"])
	assert.Equal(t, &email, event.Data["email"])
	assert.Equal(t, &phone, event.Data["phone"])
	assert.Equal(t, &timezone, event.Data["timezone"])
	assert.Equal(t, &locale, event.Data["locale"])
	assert.Equal(t, int32(5), event.Data["version"])
}

func TestUsersHandler_Success(t *testing.T) {
	t.Parallel()

	db, projectID, jet := setupUsersTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)
	handler := UsersHandler(logger, db, pub)

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

	err = pub.Publish(ctx, schemas.UsersProjectSubject(projectID), user)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamUsers, ConsumerUsers)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)
}

func TestUsersHandler_PublishesUserCreatedEvent(t *testing.T) {
	t.Parallel()

	db, projectID, jet := setupUsersTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)
	handler := UsersHandler(logger, db, pub)

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

	err = pub.Publish(ctx, schemas.UsersProjectSubject(projectID), user)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamUsers, ConsumerUsers)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	eventConsumer, err := jet.Consumer(ctx, StreamEvents, ConsumerEvents)
	require.NoError(t, err)

	eventMsg, err := eventConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	var receivedEvent schemas.Event
	err = json.Unmarshal(eventMsg.Data(), &receivedEvent)
	require.NoError(t, err)
	assert.Equal(t, schemas.EventUserCreated, receivedEvent.Name)
	assert.Equal(t, projectID, receivedEvent.ProjectID)
}

func TestUsersHandler_PublishesUserUpdatedEvent(t *testing.T) {
	t.Parallel()

	db, projectID, jet := setupUsersTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)
	handler := UsersHandler(logger, db, pub)

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

	err = pub.Publish(ctx, schemas.UsersProjectSubject(projectID), user)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamUsers, ConsumerUsers)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	eventConsumer, err := jet.Consumer(ctx, StreamEvents, ConsumerEvents)
	require.NoError(t, err)

	eventMsg, err := eventConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	var receivedEvent schemas.Event
	err = json.Unmarshal(eventMsg.Data(), &receivedEvent)
	require.NoError(t, err)
	assert.Equal(t, schemas.EventUserUpdated, receivedEvent.Name)
	assert.Equal(t, projectID, receivedEvent.ProjectID)
}

func TestUsersHandler_WithListDependencies(t *testing.T) {
	t.Parallel()

	db, projectID, jet := setupUsersTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	st := store.NewState(db)

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

	_, err = st.ListsStore.CreateList(ctx, store.List{
		ProjectID: projectID,
		Name:      "Test List",
		Type:      "static",
		RuleID:    &ruleID,
	})
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)
	handler := UsersHandler(logger, db, pub)

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

	err = pub.Publish(ctx, schemas.UsersProjectSubject(projectID), user)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamUsers, ConsumerUsers)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	recomputeConsumer, err := jet.Consumer(ctx, StreamRecompute, ConsumerRecomputeLists)
	require.NoError(t, err)

	recomputeMsg, err := recomputeConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	var recompute RecomputeList
	err = json.Unmarshal(recomputeMsg.Data(), &recompute)
	require.NoError(t, err)
	assert.Equal(t, projectID, recompute.ProjectID)
}

func TestUsersHandler_WithUserData(t *testing.T) {
	t.Parallel()

	db, projectID, jet := setupUsersTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)
	handler := UsersHandler(logger, db, pub)

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

	err = pub.Publish(ctx, schemas.UsersProjectSubject(projectID), user)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamUsers, ConsumerUsers)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	schemaConsumer, err := jet.Consumer(ctx, StreamUsers, ConsumerUserSchemas)
	require.NoError(t, err)

	schemaMsg, err := schemaConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	var receivedUser schemas.User
	err = json.Unmarshal(schemaMsg.Data(), &receivedUser)
	require.NoError(t, err)
	assert.Equal(t, user.ID, receivedUser.ID)
	assert.NotNil(t, receivedUser.Data)
}

func TestUsersHandler_WithoutData(t *testing.T) {
	t.Parallel()

	db, projectID, jet := setupUsersTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)
	handler := UsersHandler(logger, db, pub)

	email := "test@example.com"
	user := schemas.User{
		ID:        uuid.New(),
		ProjectID: projectID,
		Email:     &email,
		Data:      nil,
		Version:   0,
	}

	err = pub.Publish(ctx, schemas.UsersProjectSubject(projectID), user)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamUsers, ConsumerUsers)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	schemaConsumer, err := jet.Consumer(ctx, StreamUsers, ConsumerUserSchemas)
	require.NoError(t, err)

	_, err = schemaConsumer.Next(jetstream.FetchMaxWait(1 * time.Second))
	assert.Error(t, err)
}

func TestPublishUserRecomputeLists_Success(t *testing.T) {
	t.Parallel()

	db, projectID, jet := setupUsersTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	st := store.NewState(db)

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
		Name:      "Adult List",
		Type:      "static",
		RuleID:    &ruleID,
	})
	require.NoError(t, err)

	_, err = jet.CreateStream(ctx, jetstream.StreamConfig{
		Name:     StreamRecompute,
		Subjects: []string{"recompute.>"},
	})
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)

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

	lists := store.NewListsStore(db)
	err = PublishUserRecomputeLists(ctx, logger, lists, pub, user)
	require.NoError(t, err)

	stream, err := jet.Stream(ctx, StreamRecompute)
	require.NoError(t, err)

	info, err := stream.Info(ctx)
	require.NoError(t, err)
	assert.Greater(t, info.State.Msgs, uint64(0))

	result, err := lists.SelectListUsersDependency(ctx, projectID)
	require.NoError(t, err)
	assert.Contains(t, result, listID)
}

func TestPublishUserRecomputeLists_NoLists(t *testing.T) {
	t.Parallel()

	db, projectID, jet := setupUsersTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	_, err := jet.CreateStream(ctx, jetstream.StreamConfig{
		Name:     StreamRecompute,
		Subjects: []string{"recompute.>"},
	})
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)

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

	lists := store.NewListsStore(db)
	err = PublishUserRecomputeLists(ctx, logger, lists, pub, user)
	require.NoError(t, err)
}

func TestPublishUserEvents_UserCreated(t *testing.T) {
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

func TestPublishUserEvents_UserUpdated(t *testing.T) {
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

func TestUserSchemasHandler_Success(t *testing.T) {
	t.Parallel()

	db, projectID, jet := setupUsersTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	handler := UserSchemasHandler(logger, db)

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

	err = pubsub.NewPublisher(jet).Publish(ctx, schemas.UsersSchemaSubject(projectID), user)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamUsers, ConsumerUserSchemas)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)
}

func TestUserSchemasHandler_ComplexData(t *testing.T) {
	t.Parallel()

	db, projectID, jet := setupUsersTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	handler := UserSchemasHandler(logger, db)

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

	err = pubsub.NewPublisher(jet).Publish(ctx, schemas.UsersSchemaSubject(projectID), user)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamUsers, ConsumerUserSchemas)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)
}
