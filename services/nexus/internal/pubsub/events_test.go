package pubsub

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/pkg/container"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func setupEventsTest(t *testing.T) (*sqlx.DB, uuid.UUID, jetstream.JetStream) {
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

	jet, err := New(ctx, config)
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

func TestEventsProjectHandler_Success(t *testing.T) {
	t.Parallel()

	db, projectID, jet := setupEventsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	st := store.NewState(db)
	email := "test@example.com"
	userID, err := st.UsersStore.UpsertUser(ctx, projectID, store.UpsertUserParams{
		Email: &email,
	})
	require.NoError(t, err)

	pub := NewPublisher(jet)
	handler := EventsHandler(logger, db, pub)

	event := Event{
		Name:      "test_event",
		ProjectID: projectID,
		UserID:    userID,
		Data: &map[string]any{
			"key": "value",
		},
	}

	err = pub.Publish(ctx, EventsProjectSubject(projectID), event)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamEvents, ConsumerEvents)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)

	schemaConsumer, err := jet.Consumer(ctx, StreamEvents, ConsumerEventSchemas)
	require.NoError(t, err)

	schemaMsg, err := schemaConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	var receivedEvent Event
	err = json.Unmarshal(schemaMsg.Data(), &receivedEvent)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, receivedEvent.ID)
	assert.Equal(t, "test_event", receivedEvent.Name)
}

func TestEventsProjectHandler_WithoutData(t *testing.T) {
	t.Parallel()

	db, projectID, jet := setupEventsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	st := store.NewState(db)
	email := "test2@example.com"
	userID, err := st.UsersStore.UpsertUser(ctx, projectID, store.UpsertUserParams{
		Email: &email,
	})
	require.NoError(t, err)

	pub := NewPublisher(jet)
	handler := EventsHandler(logger, db, pub)

	event := Event{
		Name:      "test_event_no_data",
		ProjectID: projectID,
		UserID:    userID,
		Data:      nil,
	}

	err = pub.Publish(ctx, EventsProjectSubject(projectID), event)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamEvents, ConsumerEvents)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)

	schemaConsumer, err := jet.Consumer(ctx, StreamEvents, ConsumerEventSchemas)
	require.NoError(t, err)

	_, err = schemaConsumer.Next(jetstream.FetchMaxWait(1 * time.Second))
	assert.Error(t, err)
}

func TestEventsProjectHandler_WithIdentifiers(t *testing.T) {
	t.Parallel()

	db, projectID, jet := setupEventsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	st := store.NewState(db)
	externalID := "user_123"
	anonymousID := "anon_abc"

	_, err = st.UsersStore.UpsertUser(ctx, projectID, store.UpsertUserParams{
		ExternalID:  &externalID,
		AnonymousID: &anonymousID,
	})
	require.NoError(t, err)

	pub := NewPublisher(jet)
	handler := EventsHandler(logger, db, pub)

	event := Event{
		Name:        "user_action",
		ProjectID:   projectID,
		ExternalId:  &externalID,
		AnonymousId: &anonymousID,
		Data: &map[string]any{
			"action": "click",
		},
	}

	err = pub.Publish(ctx, EventsProjectSubject(projectID), event)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamEvents, ConsumerEvents)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)
}

func TestEventsSchemaHandler_Success(t *testing.T) {
	t.Parallel()

	db, projectID, jet := setupEventsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	st := store.NewState(db)
	eventID, err := st.EventsStore.UpsertEvent(ctx, projectID, "test_event")
	require.NoError(t, err)

	handler := EventSchemasHandler(logger, db)

	event := Event{
		ID:        eventID,
		Name:      "test_event",
		ProjectID: projectID,
		Data: &map[string]any{
			"user": map[string]any{
				"name":  "John",
				"email": "john@example.com",
			},
			"amount": 99.99,
		},
	}

	err = NewPublisher(jet).Publish(ctx, EventsSchemaSubject(projectID), event)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamEvents, ConsumerEventSchemas)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)
}

func TestEventsSchemaHandler_ComplexNestedData(t *testing.T) {
	t.Parallel()

	db, projectID, jet := setupEventsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	st := store.NewState(db)
	eventID, err := st.EventsStore.UpsertEvent(ctx, projectID, "complex_event")
	require.NoError(t, err)

	handler := EventSchemasHandler(logger, db)

	event := Event{
		ID:        eventID,
		Name:      "complex_event",
		ProjectID: projectID,
		Data: &map[string]any{
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

	err = NewPublisher(jet).Publish(ctx, EventsSchemaSubject(projectID), event)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, StreamEvents, ConsumerEventSchemas)
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)
}
