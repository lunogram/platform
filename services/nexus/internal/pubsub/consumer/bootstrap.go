package consumer

import (
	"context"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

func Bootstrap(ctx graceful.Context, logger *zap.Logger, jet jetstream.JetStream) error {
	logger.Info("bootstrapping pubsub streams and consumers...")
	bootstrap := NewBootstrapper(logger, jet)

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        StreamUsers,
		Description: "Responsible for receiving incoming users",
		Subjects:    []string{"users.>"},
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})

	bootstrap.EnsureConsumer(ctx, StreamUsers, jetstream.ConsumerConfig{
		Name:          ConsumerUsers,
		FilterSubject: "users.projects.>",
		Description:   "Processes incoming users",
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    5,
	})

	bootstrap.EnsureConsumer(ctx, StreamUsers, jetstream.ConsumerConfig{
		Name:          ConsumerUserSchemas,
		FilterSubject: "users.schemas.>",
		Description:   "Processes user schema definitions",
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    5,
	})

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        StreamEvents,
		Description: "Responsible for receiving incoming events",
		Subjects:    []string{"events.>"},
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})

	bootstrap.EnsureConsumer(ctx, StreamEvents, jetstream.ConsumerConfig{
		Name:          ConsumerEvents,
		FilterSubject: "events.projects.>",
		Description:   "Processes incoming events",
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    5,
	})

	bootstrap.EnsureConsumer(ctx, StreamEvents, jetstream.ConsumerConfig{
		Name:          ConsumerEventSchemas,
		FilterSubject: "events.schemas.>",
		Description:   "Processes event schema definitions",
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    5,
	})

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        StreamRecompute,
		Description: "Recompute triggers for derived data",
		Subjects: []string{
			"recompute.lists.>",
		},
		Discard:  jetstream.DiscardOld,
		MaxAge:   24 * time.Hour,
		Replicas: 1,
	})

	bootstrap.EnsureConsumer(ctx, StreamRecompute, jetstream.ConsumerConfig{
		Name:          ConsumerRecomputeLists,
		Description:   "Processes recompute list requests",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: "recompute.lists.>",
		MaxDeliver:    5,
	})

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        StreamJourney,
		Description: "Advance a journey state based on incoming events",
		Subjects: []string{
			"journeys.state.>",
		},
		Discard:  jetstream.DiscardOld,
		MaxAge:   24 * time.Hour,
		Replicas: 1,
	})

	bootstrap.EnsureConsumer(ctx, StreamJourney, jetstream.ConsumerConfig{
		Name:          ConsumerJourneysState,
		Description:   "Processes journey state requests",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: "journeys.state.>",
		MaxDeliver:    5,
	})

	return bootstrap.Error()
}

func NewBootstrapper(logger *zap.Logger, jet jetstream.JetStream) *Bootstrapper {
	return &Bootstrapper{
		jet:    jet,
		logger: logger,
	}
}

type Bootstrapper struct {
	err    error
	jet    jetstream.JetStream
	logger *zap.Logger
}

func (b *Bootstrapper) EnsureStream(ctx graceful.Context, config jetstream.StreamConfig) {
	if b.err != nil {
		return
	}

	b.logger.Info("ensuring stream", zap.String("stream", config.Name))

	_, b.err = b.jet.Stream(ctx, config.Name)
	if b.err != nil && b.err != jetstream.ErrStreamNotFound {
		b.logger.Error("error checking for stream", zap.String("stream", config.Name), zap.Error(b.err))
		return
	}

	if b.err == nil {
		b.logger.Info("stream already exists", zap.String("stream", config.Name))
		return
	}

	b.logger.Info("creating stream", zap.String("stream", config.Name))
	_, b.err = b.jet.CreateStream(ctx, config)
}

func (b *Bootstrapper) EnsureConsumer(ctx context.Context, stream string, config jetstream.ConsumerConfig) {
	if b.err != nil {
		return
	}

	b.logger.Info("ensuring consumer", zap.String("stream", stream), zap.String("consumer", config.Name))

	if config.Name == "" {
		config.Name = config.Durable
	}

	if config.Durable == "" {
		config.Durable = config.Name
	}

	_, b.err = b.jet.Consumer(ctx, stream, config.Durable)
	if b.err != nil && b.err != jetstream.ErrConsumerNotFound {
		b.logger.Error("error checking for consumer", zap.String("stream", stream), zap.String("consumer", config.Durable), zap.Error(b.err))
		return
	}

	if b.err == nil {
		b.logger.Info("consumer already exists", zap.String("stream", stream), zap.String("consumer", config.Durable))
		return
	}

	b.logger.Info("creating consumer", zap.String("stream", stream), zap.String("consumer", config.Durable))
	_, b.err = b.jet.CreateConsumer(ctx, stream, config)
}

func (b *Bootstrapper) Error() error {
	return b.err
}
