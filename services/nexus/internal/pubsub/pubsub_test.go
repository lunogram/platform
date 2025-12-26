package pubsub

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/pkg/container"
	"github.com/lunogram/platform/services/nexus/internal/config"
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
	pub := NewPublisher(jet)

	assert.NotNil(t, pub)
}

func TestPublisher_Publish(t *testing.T) {
	t.Parallel()

	jet := setupJetStream(t)
	ctx := graceful.NewContext(t.Context())

	// Create a stream for testing
	_, err := jet.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "test_events",
		Subjects: []string{"events.projects.>"},
	})
	require.NoError(t, err)

	pub := NewPublisher(jet)

	type test struct {
		subject Subject
		data    any
	}

	tests := map[string]test{
		"publish event": {
			subject: "events.projects.123",
			data: Event{
				ID:        uuid.New(),
				Name:      "test_event",
				ProjectID: uuid.New(),
			},
		},
		"publish with nil data": {
			subject: "events.projects.456",
			data: Event{
				ID:          uuid.New(),
				Name:        "another_event",
				ProjectID:   uuid.New(),
				AnonymousId: nil,
			},
		},
		"publish with event data": {
			subject: "events.projects.789",
			data: Event{
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

func TestEventsProject(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	subject := EventsProjectSubject(projectID)

	expected := Subject("events.projects." + projectID.String())
	assert.Equal(t, expected, subject)
}

func TestEventsSchema(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	subject := EventsSchemaSubject(projectID)

	expected := Subject("events.schemas." + projectID.String())
	assert.Equal(t, expected, subject)
}

func TestPublisher_PublishAndReceive(t *testing.T) {
	t.Parallel()

	jet := setupJetStream(t)
	ctx := graceful.NewContext(t.Context())

	_, err := jet.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "test_stream",
		Subjects: []string{"test.>"},
	})
	require.NoError(t, err)

	pub := NewPublisher(jet)

	testEvent := Event{
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

	var received Event
	err = json.Unmarshal(msg.Data(), &received)
	require.NoError(t, err)

	assert.Equal(t, testEvent.ID, received.ID)
	assert.Equal(t, testEvent.Name, received.Name)
	assert.Equal(t, testEvent.ProjectID, received.ProjectID)
}
