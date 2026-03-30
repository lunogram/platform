package consumer

import (
	"strings"
	"testing"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// testNamespace returns a unique Namespace derived from the test name.
// Slashes (from subtests) and dots (NATS subject delimiters) are replaced
// with underscores so the namespace is safe for stream, consumer, and
// subject names.
func testNamespace(t *testing.T) Namespace {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "_")
	name = strings.ReplaceAll(name, ".", "_")
	return Namespace(name)
}

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

func TestBootstrapCreatesStreamsAndConsumers(t *testing.T) {
	t.Parallel()

	jet := setupBootstrapTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	ns := testNamespace(t)

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	stream, err := jet.Stream(ctx, ns.Stream(StreamUserEvents))
	require.NoError(t, err)
	assert.NotNil(t, stream)

	info, err := stream.Info(ctx)
	require.NoError(t, err)
	assert.Equal(t, ns.Stream(StreamUserEvents), info.Config.Name)
	assert.Contains(t, info.Config.Subjects, ns.Subject("users.events.>"))

	consumer, err := jet.Consumer(ctx, ns.Stream(StreamUserEvents), ns.Consumer(ConsumerUserEventsProcess))
	require.NoError(t, err)
	assert.NotNil(t, consumer)

	consumerInfo, err := consumer.Info(ctx)
	require.NoError(t, err)
	assert.Equal(t, ns.Consumer(ConsumerUserEventsProcess), consumerInfo.Name)
	assert.Equal(t, ns.Subject("users.events.process.>"), consumerInfo.Config.FilterSubject)
}

func TestBootstrapCreatesRecomputeStream(t *testing.T) {
	t.Parallel()

	jet := setupBootstrapTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	ns := testNamespace(t)

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	stream, err := jet.Stream(ctx, ns.Stream(StreamLists))
	require.NoError(t, err)
	assert.NotNil(t, stream)

	info, err := stream.Info(ctx)
	require.NoError(t, err)
	assert.Equal(t, ns.Stream(StreamLists), info.Config.Name)
	assert.Contains(t, info.Config.Subjects, ns.Subject("lists.>"))
}

func TestBootstrapCreatesAllConsumers(t *testing.T) {
	t.Parallel()

	jet := setupBootstrapTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	ns := testNamespace(t)

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	type consumer struct {
		stream   string
		consumer string
		filter   string
	}

	consumers := []consumer{
		{ns.Stream(StreamUserEvents), ns.Consumer(ConsumerUserEventsProcess), ns.Subject("users.events.process.>")},
		{ns.Stream(StreamUserEvents), ns.Consumer(ConsumerUserEventsSchema), ns.Subject("users.events.schema.>")},
		{ns.Stream(StreamJourneys), ns.Consumer(ConsumerJourneysAdvance), ns.Subject("journeys.advance.>")},
		{ns.Stream(StreamLists), ns.Consumer(ConsumerListsRecompute), ns.Subject("lists.recompute.>")},
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

func TestBootstrapIdempotent(t *testing.T) {
	t.Parallel()

	jet := setupBootstrapTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	ns := testNamespace(t)

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	err = Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	stream, err := jet.Stream(ctx, ns.Stream(StreamUserEvents))
	require.NoError(t, err)
	assert.NotNil(t, stream)
}

func TestBootstrapperEnsureStream(t *testing.T) {
	t.Parallel()

	jet := setupBootstrapTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	bootstrapper := NewBootstrapper(logger, jet, false)

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

func TestBootstrapperEnsureConsumer(t *testing.T) {
	t.Parallel()

	jet := setupBootstrapTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())

	_, err := jet.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "test_stream",
		Subjects: []string{"test.>"},
	})
	require.NoError(t, err)

	bootstrapper := NewBootstrapper(logger, jet, false)

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

func TestBootstrapperStreamRetention(t *testing.T) {
	t.Parallel()

	jet := setupBootstrapTest(t)
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	ns := testNamespace(t)

	err := Bootstrap(ctx, logger, jet, ns)
	require.NoError(t, err)

	stream, err := jet.Stream(ctx, ns.Stream(StreamUserEvents))
	require.NoError(t, err)

	info, err := stream.Info(ctx)
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, info.Config.MaxAge)
	assert.Equal(t, jetstream.DiscardOld, info.Config.Discard)
}
