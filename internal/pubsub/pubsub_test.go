package pubsub

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupJetStream(t *testing.T) jetstream.JetStream {
	t.Helper()

	natsURL := container.RunNATS(t)
	ctx := graceful.NewContext(t.Context())

	jet, err := New(ctx, config.Node{
		Nats: config.Nats{
			URL: natsURL,
		},
	})
	require.NoError(t, err)

	return jet
}

func TestNewPublisher(t *testing.T) {
	t.Parallel()

	jet := setupJetStream(t)
	pub := NewPublisher(jet, "")

	assert.NotNil(t, pub)
}

func TestPublisherPublish(t *testing.T) {
	t.Parallel()

	jet := setupJetStream(t)
	ctx := graceful.NewContext(t.Context())

	// Create a stream for testing
	_, err := jet.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "test_events",
		Subjects: []string{"events.process.>"},
	})
	require.NoError(t, err)

	pub := NewPublisher(jet, "")

	type test struct {
		subject schemas.Subject
		data    any
	}

	tests := map[string]test{
		"publish event": {
			subject: "events.process.123",
			data: schemas.UserEvent{
				ID:        uuid.New(),
				Name:      "test_event",
				ProjectID: uuid.New(),
			},
		},
		"publish with nil data": {
			subject: "events.process.456",
			data: schemas.UserEvent{
				ID:          uuid.New(),
				Name:        "another_event",
				ProjectID:   uuid.New(),
				AnonymousId: nil,
			},
		},
		"publish with event data": {
			subject: "events.process.789",
			data: schemas.UserEvent{
				ID:        uuid.New(),
				Name:      "event_with_data",
				ProjectID: uuid.New(),
				Data: map[string]any{
					"key": "value",
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			err := pub.Publish(ctx, tc.subject, tc.data)
			require.NoError(t, err)
		})
	}
}

func TestUserEventsProcess(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	subject := schemas.UserEventsProcess(projectID)

	expected := schemas.Subject("users.events.process." + projectID.String())
	assert.Equal(t, expected, subject)
}

func TestUserEventsSchema(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	subject := schemas.UserEventsSchema(projectID)

	expected := schemas.Subject("users.events.schema." + projectID.String())
	assert.Equal(t, expected, subject)
}

func TestPublisherPublishAndReceive(t *testing.T) {
	t.Parallel()

	jet := setupJetStream(t)
	ctx := graceful.NewContext(t.Context())

	_, err := jet.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "test_stream",
		Subjects: []string{"test.>"},
	})
	require.NoError(t, err)

	pub := NewPublisher(jet, "")

	testEvent := schemas.UserEvent{
		ID:        uuid.New(),
		Name:      "test_event",
		ProjectID: uuid.New(),
		Data: map[string]any{
			"key": "value",
		},
	}

	err = pub.Publish(ctx, "test.event", testEvent)
	require.NoError(t, err)

	consumer, err := jet.CreateConsumer(ctx, "test_stream", jetstream.ConsumerConfig{
		Durable:   "test_consumer",
		AckPolicy: jetstream.AckExplicitPolicy,
	})
	require.NoError(t, err)

	msg, err := consumer.Next()
	require.NoError(t, err)

	var received schemas.UserEvent
	err = json.Unmarshal(msg.Data(), &received)
	require.NoError(t, err)

	assert.Equal(t, testEvent.ID, received.ID)
	assert.Equal(t, testEvent.Name, received.Name)
	assert.Equal(t, testEvent.ProjectID, received.ProjectID)
}
