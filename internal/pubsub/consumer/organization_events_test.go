package consumer

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func TestOrganizationEventsHandlerSuccess(t *testing.T) {
	t.Parallel()

	_, usrsDB, jrnyDB, projectID, jet := setupOrganizationsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	ns := testNamespace(t)

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	usersState := subjects.NewState(usrsDB, zap.NewNop())
	journeyState := journey.NewState(jrnyDB)
	pub := pubsub.NewPublisher(jet, string(ns))

	// First, create an organization in the database
	orgID, err := usersState.UpsertOrganization(ctx, projectID, subjects.UpsertOrganizationParams{
		ExternalID: "org_event_test",
		Name:       strPtr("Test Org for Events"),
	})
	require.NoError(t, err)

	handler := OrganizationEventsHandler(logger, usersState, journeyState, pub)

	event := schemas.OrganizationEvent{
		Name:           "purchase.completed",
		ProjectID:      projectID,
		OrganizationID: orgID,
		Data: map[string]any{
			"amount":   999.99,
			"currency": "USD",
		},
	}

	err = pub.Publish(ctx, schemas.OrganizationEventsProcess(projectID), event)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamOrganizationEvents), ns.Consumer(ConsumerOrganizationEventsProcess))
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)

	// Verify schema message was published (since Data was provided)
	schemaConsumer, err := jet.Consumer(ctx, ns.Stream(StreamOrganizationEvents), ns.Consumer(ConsumerOrganizationEventsSchema))
	require.NoError(t, err)

	schemaMsg, err := schemaConsumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	var receivedEvent schemas.OrganizationEvent
	err = json.Unmarshal(schemaMsg.Data(), &receivedEvent)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, receivedEvent.ID)
	assert.Equal(t, "purchase.completed", receivedEvent.Name)
}

func TestOrganizationEventsHandlerWithExternalID(t *testing.T) {
	t.Parallel()

	_, usrsDB, jrnyDB, projectID, jet := setupOrganizationsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	ns := testNamespace(t)

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	usersState := subjects.NewState(usrsDB, zap.NewNop())
	journeyState := journey.NewState(jrnyDB)
	pub := pubsub.NewPublisher(jet, string(ns))

	// Create an organization with a known external ID
	externalID := "org_external_123"
	_, err = usersState.UpsertOrganization(ctx, projectID, subjects.UpsertOrganizationParams{
		ExternalID: externalID,
		Name:       strPtr("External ID Org"),
	})
	require.NoError(t, err)

	handler := OrganizationEventsHandler(logger, usersState, journeyState, pub)

	// Send event using external ID instead of internal ID
	event := schemas.OrganizationEvent{
		Name:                   "subscription.renewed",
		ProjectID:              projectID,
		OrganizationID:         uuid.Nil, // No internal ID
		OrganizationExternalID: externalID,
		Data: map[string]any{
			"plan": "enterprise",
		},
	}

	err = pub.Publish(ctx, schemas.OrganizationEventsProcess(projectID), event)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamOrganizationEvents), ns.Consumer(ConsumerOrganizationEventsProcess))
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)
}

func TestOrganizationEventsHandlerWithoutData(t *testing.T) {
	t.Parallel()

	_, usrsDB, jrnyDB, projectID, jet := setupOrganizationsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	ns := testNamespace(t)

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	usersState := subjects.NewState(usrsDB, zap.NewNop())
	journeyState := journey.NewState(jrnyDB)
	pub := pubsub.NewPublisher(jet, string(ns))

	// Create an organization
	orgID, err := usersState.UpsertOrganization(ctx, projectID, subjects.UpsertOrganizationParams{
		ExternalID: "org_no_data_event",
		Name:       strPtr("No Data Event Org"),
	})
	require.NoError(t, err)

	handler := OrganizationEventsHandler(logger, usersState, journeyState, pub)

	event := schemas.OrganizationEvent{
		Name:           "org.activated",
		ProjectID:      projectID,
		OrganizationID: orgID,
		Data:           nil, // No data
	}

	err = pub.Publish(ctx, schemas.OrganizationEventsProcess(projectID), event)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamOrganizationEvents), ns.Consumer(ConsumerOrganizationEventsProcess))
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)

	// Verify NO schema message was published (since Data is nil)
	schemaConsumer, err := jet.Consumer(ctx, ns.Stream(StreamOrganizationEvents), ns.Consumer(ConsumerOrganizationEventsSchema))
	require.NoError(t, err)

	_, err = schemaConsumer.Next(jetstream.FetchMaxWait(1 * time.Second))
	assert.Error(t, err) // Should timeout
}

func TestOrganizationEventSchemasHandlerSuccess(t *testing.T) {
	t.Parallel()

	_, usrsDB, _, projectID, jet := setupOrganizationsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	ns := testNamespace(t)

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	usersState := subjects.NewState(usrsDB, zap.NewNop())

	// First, create the event in the database
	eventID, err := usersState.UpsertEvent(ctx, projectID, "org.contract.signed", subjects.SubjectTypeOrganization)
	require.NoError(t, err)

	handler := OrganizationEventSchemasHandler(logger, usersState)

	event := schemas.OrganizationEvent{
		ID:        eventID,
		Name:      "org.contract.signed",
		ProjectID: projectID,
		Data: map[string]any{
			"contract": map[string]any{
				"id":       "contract_123",
				"value":    50000.00,
				"duration": "12 months",
			},
			"signed_by": "CEO",
		},
	}

	err = pubsub.NewPublisher(jet, string(ns)).Publish(ctx, schemas.OrganizationEventsSchema(projectID), event)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamOrganizationEvents), ns.Consumer(ConsumerOrganizationEventsSchema))
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)

	err = msg.Ack()
	require.NoError(t, err)
}

func TestOrganizationEventSchemasHandlerComplexNestedData(t *testing.T) {
	t.Parallel()

	_, usrsDB, _, projectID, jet := setupOrganizationsTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	ns := testNamespace(t)

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	usersState := subjects.NewState(usrsDB, zap.NewNop())

	// Create event
	eventID, err := usersState.UpsertEvent(ctx, projectID, "org.deal.closed", subjects.SubjectTypeOrganization)
	require.NoError(t, err)

	handler := OrganizationEventSchemasHandler(logger, usersState)

	event := schemas.OrganizationEvent{
		ID:        eventID,
		Name:      "org.deal.closed",
		ProjectID: projectID,
		Data: map[string]any{
			"deal": map[string]any{
				"id":     "deal_456",
				"amount": 100000.00,
				"products": []map[string]any{
					{"name": "Enterprise License", "quantity": 10},
					{"name": "Support Package", "quantity": 1},
				},
				"metadata": map[string]any{
					"source":    "inbound",
					"rep_id":    "sales_rep_789",
					"closed_at": "2025-02-22T10:00:00Z",
				},
			},
			"customer_type": "enterprise",
		},
	}

	err = pubsub.NewPublisher(jet, string(ns)).Publish(ctx, schemas.OrganizationEventsSchema(projectID), event)
	require.NoError(t, err)

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamOrganizationEvents), ns.Consumer(ConsumerOrganizationEventsSchema))
	require.NoError(t, err)

	msg, err := consumer.Next(jetstream.FetchMaxWait(5 * time.Second))
	require.NoError(t, err)

	err = handler(ctx, msg)
	require.NoError(t, err)
}
