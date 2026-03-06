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
		Name:          ConsumerUsersProcess,
		FilterSubject: "users.process.>",
		Description:   "Processes incoming users",
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    5,
	})

	bootstrap.EnsureConsumer(ctx, StreamUsers, jetstream.ConsumerConfig{
		Name:          ConsumerUsersSchema,
		FilterSubject: "users.schema.>",
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
		Name:          ConsumerEventsProcess,
		FilterSubject: "events.process.>",
		Description:   "Processes incoming events",
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    5,
	})

	bootstrap.EnsureConsumer(ctx, StreamEvents, jetstream.ConsumerConfig{
		Name:          ConsumerEventsSchema,
		FilterSubject: "events.schema.>",
		Description:   "Processes event schema definitions",
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    5,
	})

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        StreamLists,
		Description: "List recomputation triggers",
		Subjects:    []string{"lists.>"},
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})

	bootstrap.EnsureConsumer(ctx, StreamLists, jetstream.ConsumerConfig{
		Name:          ConsumerListsRecompute,
		Description:   "Processes list recomputation requests",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: "lists.recompute.>",
		MaxDeliver:    5,
	})

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        StreamJourneys,
		Description: "Journey advancement and orchestration",
		Subjects:    []string{"journeys.>"},
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})

	bootstrap.EnsureConsumer(ctx, StreamJourneys, jetstream.ConsumerConfig{
		Name:          ConsumerJourneysAdvance,
		Description:   "Processes journey advancement requests",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: "journeys.advance.>",
		MaxDeliver:    5,
	})

	bootstrap.EnsureConsumer(ctx, StreamJourneys, jetstream.ConsumerConfig{
		Name:          ConsumerJourneysAdvanceUser,
		Description:   "Processes journey advancement requests per specific user",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: "journeys.advance.>",
		MaxDeliver:    5,
	})

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        StreamCampaigns,
		Description: "Campaign sending and execution",
		Subjects:    []string{"campaigns.>"},
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})

	bootstrap.EnsureConsumer(ctx, StreamCampaigns, jetstream.ConsumerConfig{
		Name:          ConsumerCampaignsSend,
		Description:   "Processes campaign send requests",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: "campaigns.send.>",
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
