package consumer

import (
	"testing"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/pkg/container"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/pubsub"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func setupBootstrapTest(t *testing.T) jetstream.JetStream {
	t.Helper()

	natsURL := container.RunNATS(t)
	ctx := graceful.NewContext(t.Context())

	jet, err := pubsub.New(ctx, config.Node{
		Nats: config.Nats{
			URL: natsURL,
		},
	})
	require.NoError(t, err)

	return jet
}

func TestBootstrap_CreatesStreamsAndConsumers(t *testing.T) {
	t.Parallel()

	jet := setupBootstrapTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	stream, err := jet.Stream(ctx, StreamEvents)
	require.NoError(t, err)
	assert.NotNil(t, stream)

	info, err := stream.Info(ctx)
	require.NoError(t, err)
	assert.Equal(t, StreamEvents, info.Config.Name)
	assert.Contains(t, info.Config.Subjects, "events.>")

	consumer, err := jet.Consumer(ctx, StreamEvents, ConsumerEvents)
	require.NoError(t, err)
	assert.NotNil(t, consumer)

	consumerInfo, err := consumer.Info(ctx)
	require.NoError(t, err)
	assert.Equal(t, ConsumerEvents, consumerInfo.Name)
	assert.Equal(t, "events.projects.>", consumerInfo.Config.FilterSubject)
}

func TestBootstrap_CreatesRecomputeStream(t *testing.T) {
	t.Parallel()

	jet := setupBootstrapTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	stream, err := jet.Stream(ctx, StreamRecompute)
	require.NoError(t, err)
	assert.NotNil(t, stream)

	info, err := stream.Info(ctx)
	require.NoError(t, err)
	assert.Equal(t, StreamRecompute, info.Config.Name)
	assert.Contains(t, info.Config.Subjects, "recompute.lists.>")
}

func TestBootstrap_CreatesAllConsumers(t *testing.T) {
	t.Parallel()

	jet := setupBootstrapTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	type consumer struct {
		stream   string
		consumer string
		filter   string
	}

	consumers := []consumer{
		{StreamEvents, ConsumerEvents, "events.projects.>"},
		{StreamEvents, ConsumerEventSchemas, "events.schemas.>"},
		{StreamJourney, ConsumerJourneysState, "journeys.state.>"},
		{StreamRecompute, ConsumerRecomputeLists, "recompute.lists.>"},
	}

	for _, tc := range consumers {
		consumer, err := jet.Consumer(ctx, tc.stream, tc.consumer)
		require.NoError(t, err, "consumer %s should exist", tc.consumer)

		info, err := consumer.Info(ctx)
		require.NoError(t, err)
		assert.Equal(t, tc.consumer, info.Name)
		assert.Equal(t, tc.filter, info.Config.FilterSubject)
	}
}

func TestBootstrap_Idempotent(t *testing.T) {
	t.Parallel()

	jet := setupBootstrapTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	err = Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	stream, err := jet.Stream(ctx, StreamEvents)
	require.NoError(t, err)
	assert.NotNil(t, stream)
}

func TestBootstrapper_EnsureStream(t *testing.T) {
	t.Parallel()

	jet := setupBootstrapTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	bootstrapper := NewBootstrapper(logger, jet)

	config := jetstream.StreamConfig{
		Name:        "test_stream",
		Description: "Test stream",
		Subjects:    []string{"test.>"},
		MaxAge:      24 * time.Hour,
	}

	bootstrapper.EnsureStream(ctx, config)
	require.NoError(t, bootstrapper.Error())

	stream, err := jet.Stream(ctx, "test_stream")
	require.NoError(t, err)
	assert.NotNil(t, stream)
}

func TestBootstrapper_EnsureConsumer(t *testing.T) {
	t.Parallel()

	jet := setupBootstrapTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	_, err := jet.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "test_stream",
		Subjects: []string{"test.>"},
	})
	require.NoError(t, err)

	bootstrapper := NewBootstrapper(logger, jet)

	config := jetstream.ConsumerConfig{
		Name:          "test_consumer",
		FilterSubject: "test.>",
		AckPolicy:     jetstream.AckExplicitPolicy,
	}

	bootstrapper.EnsureConsumer(ctx, "test_stream", config)
	require.NoError(t, bootstrapper.Error())

	consumer, err := jet.Consumer(ctx, "test_stream", "test_consumer")
	require.NoError(t, err)
	assert.NotNil(t, consumer)
}

func TestBootstrapper_StreamRetention(t *testing.T) {
	t.Parallel()

	jet := setupBootstrapTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	err := Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	stream, err := jet.Stream(ctx, StreamEvents)
	require.NoError(t, err)

	info, err := stream.Info(ctx)
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, info.Config.MaxAge)
	assert.Equal(t, jetstream.DiscardOld, info.Config.Discard)
}
