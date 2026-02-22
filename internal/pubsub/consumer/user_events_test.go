package consumer

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func setupUserEventsTest(t *testing.T) (mgmtDB, usrsDB, jrnyDB *sqlx.DB, projectID uuid.UUID, jet jetstream.JetStream) {
	t.Helper()

	ctx := graceful.NewContext(t.Context())

	natsURL := container.RunNATS(t)
	mgmtDB, usrsDB, jrnyDB = teststore.RunPostgreSQL(t)

	cfg := config.Node{
		Nats: config.Nats{
			URL: natsURL,
		},
	}

	jet, err := pubsub.New(ctx, cfg)
	require.NoError(t, err)

	mgmtState := management.NewState(mgmtDB)

	orgID, err := mgmtState.OrganizationsStore.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projectID, err = mgmtState.ProjectsStore.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	return mgmtDB, usrsDB, jrnyDB, projectID, jet
}

func TestUserEventsProjectHandlerSuccess(t *testing.T) {
	t.Parallel()

	_, usrsDB, jrnyDB, projectID, jet := setupUserEventsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	usersState := subjects.NewState(usrsDB)
	journeyState := journey.NewState(jrnyDB)
	email := "test@example.com"
	userID, err := usersState.UsersStore.UpsertUser(ctx, projectID, subjects.UpsertUserParams{
		Email: &email,
	})
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)
	handler := UserEventsHandler(logger, usersState, journeyState, pub)

	event := schemas.UserEvent{
		Name:      "test_event",
		ProjectID: projectID,
		UserID:    userID,
		Data: map[string]any{
			"key": "value",
		},
	}

	err = pub.Publish(ctx, schemas.Subject(schemas.UserEventsProcess(projectID)), event)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamUserEvents, ConsumerUserEventsProcess)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)

	schemaConsumer, err := jet.Consumer(ctx, StreamUserEvents, ConsumerUserEventsSchema)
	require.NoError(t, err)

	schemaMsg, err := schemaConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	var receivedEvent schemas.UserEvent
	err = json.Unmarshal(schemaMsg.Data(), &receivedEvent)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, receivedEvent.ID)
	assert.Equal(t, "test_event", receivedEvent.Name)
}

func TestUserEventsProjectHandlerWithoutData(t *testing.T) {
	t.Parallel()

	_, usrsDB, jrnyDB, projectID, jet := setupUserEventsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	usersState := subjects.NewState(usrsDB)
	journeyState := journey.NewState(jrnyDB)
	email := "test2@example.com"
	userID, err := usersState.UsersStore.UpsertUser(ctx, projectID, subjects.UpsertUserParams{
		Email: &email,
	})
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)
	handler := UserEventsHandler(logger, usersState, journeyState, pub)

	event := schemas.UserEvent{
		Name:      "test_event_no_data",
		ProjectID: projectID,
		UserID:    userID,
		Data:      nil,
	}

	err = pub.Publish(ctx, schemas.Subject(schemas.UserEventsProcess(projectID)), event)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamUserEvents, ConsumerUserEventsProcess)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)

	schemaConsumer, err := jet.Consumer(ctx, StreamUserEvents, ConsumerUserEventsSchema)
	require.NoError(t, err)

	_, err = schemaConsumer.Next(jetstream.FetchMaxWait(1 * time.Second))
	assert.Error(t, err)
}

func TestUserEventsProjectHandlerWithIdentifiers(t *testing.T) {
	t.Parallel()

	_, usrsDB, jrnyDB, projectID, jet := setupUserEventsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	usersState := subjects.NewState(usrsDB)
	journeyState := journey.NewState(jrnyDB)
	externalID := "user_123"
	anonymousID := "anon_abc"

	_, err = usersState.UsersStore.UpsertUser(ctx, projectID, subjects.UpsertUserParams{
		ExternalID:  &externalID,
		AnonymousID: &anonymousID,
	})
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)
	handler := UserEventsHandler(logger, usersState, journeyState, pub)

	event := schemas.UserEvent{
		Name:        "user_action",
		ProjectID:   projectID,
		ExternalId:  &externalID,
		AnonymousId: &anonymousID,
		Data: map[string]any{
			"action": "click",
		},
	}

	err = pub.Publish(ctx, schemas.Subject(schemas.UserEventsProcess(projectID)), event)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamUserEvents, ConsumerUserEventsProcess)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)
}

func TestUserEventsSchemaHandlerSuccess(t *testing.T) {
	t.Parallel()

	_, usrsDB, _, projectID, jet := setupUserEventsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	usersState := subjects.NewState(usrsDB)
	eventID, err := usersState.EventsStore.UpsertEvent(ctx, projectID, "test_event")
	require.NoError(t, err)

	handler := UserEventSchemasHandler(logger, usersState)

	event := schemas.UserEvent{
		ID:        eventID,
		Name:      "test_event",
		ProjectID: projectID,
		Data: map[string]any{
			"user": map[string]any{
				"name":  "John",
				"email": "john@example.com",
			},
			"amount": 99.99,
		},
	}

	err = pubsub.NewPublisher(jet).Publish(ctx, schemas.Subject(schemas.UserEventsSchema(projectID)), event)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamUserEvents, ConsumerUserEventsSchema)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)
}

func TestUserEventsSchemaHandlerComplexNestedData(t *testing.T) {
	t.Parallel()

	_, usrsDB, _, projectID, jet := setupUserEventsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	usersState := subjects.NewState(usrsDB)
	eventID, err := usersState.EventsStore.UpsertEvent(ctx, projectID, "complex_event")
	require.NoError(t, err)

	handler := UserEventSchemasHandler(logger, usersState)

	event := schemas.UserEvent{
		ID:        eventID,
		Name:      "complex_event",
		ProjectID: projectID,
		Data: map[string]any{
			"product": map[string]any{
				"id":    "prod_123",
				"name":  "Widget",
				"price": 99.99,
				"metadata": map[string]any{
					"sku":      "WDG-001",
					"category": "electronics",
				},
			},
			"quantity": 2,
			"tags":     []string{"sale", "featured"},
		},
	}

	err = pubsub.NewPublisher(jet).Publish(ctx, schemas.Subject(schemas.UserEventsSchema(projectID)), event)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamUserEvents, ConsumerUserEventsSchema)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)
}
