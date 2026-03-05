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
		Subjects:    []string{"users.process.>", "users.schema.>"},
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
		Name:        StreamUserEvents,
		Description: "Responsible for receiving incoming user events",
		Subjects:    []string{"users.events.>"},
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})

	bootstrap.EnsureConsumer(ctx, StreamUserEvents, jetstream.ConsumerConfig{
		Name:          ConsumerUserEventsProcess,
		FilterSubject: "users.events.process.>",
		Description:   "Processes incoming user events",
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    5,
	})

	bootstrap.EnsureConsumer(ctx, StreamUserEvents, jetstream.ConsumerConfig{
		Name:          ConsumerUserEventsSchema,
		FilterSubject: "users.events.schema.>",
		Description:   "Processes user event schema definitions",
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

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        StreamOrganizations,
		Description: "Organization processing and schema extraction",
		Subjects:    []string{"organizations.process.>", "organizations.schema.>"},
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})

	bootstrap.EnsureConsumer(ctx, StreamOrganizations, jetstream.ConsumerConfig{
		Name:          ConsumerOrganizationsProcess,
		Description:   "Processes incoming organizations",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: "organizations.process.>",
		MaxDeliver:    5,
	})

	bootstrap.EnsureConsumer(ctx, StreamOrganizations, jetstream.ConsumerConfig{
		Name:          ConsumerOrganizationsSchema,
		Description:   "Processes organization schema definitions",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: "organizations.schema.>",
		MaxDeliver:    5,
	})

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        StreamOrganizationUsers,
		Description: "Organization user membership processing",
		Subjects:    []string{"organizations.users.>"},
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})

	bootstrap.EnsureConsumer(ctx, StreamOrganizationUsers, jetstream.ConsumerConfig{
		Name:          ConsumerOrganizationUsersProcess,
		Description:   "Processes organization user memberships",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: "organizations.users.process.>",
		MaxDeliver:    5,
	})

	bootstrap.EnsureConsumer(ctx, StreamOrganizationUsers, jetstream.ConsumerConfig{
		Name:          ConsumerOrganizationUsersSchema,
		Description:   "Processes organization user schema definitions",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: "organizations.users.schema.>",
		MaxDeliver:    5,
	})

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        StreamOrganizationEvents,
		Description: "Organization event processing",
		Subjects:    []string{"organizations.events.>"},
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})

	bootstrap.EnsureConsumer(ctx, StreamOrganizationEvents, jetstream.ConsumerConfig{
		Name:          ConsumerOrganizationEventsProcess,
		Description:   "Processes incoming organization events",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: "organizations.events.process.>",
		MaxDeliver:    5,
	})

	bootstrap.EnsureConsumer(ctx, StreamOrganizationEvents, jetstream.ConsumerConfig{
		Name:          ConsumerOrganizationEventsSchema,
		Description:   "Processes organization event schema definitions",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: "organizations.events.schema.>",
		MaxDeliver:    5,
	})

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        StreamActions,
		Description: "Action schema extraction",
		Subjects:    []string{"actions.schema.>"},
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})

	bootstrap.EnsureConsumer(ctx, StreamActions, jetstream.ConsumerConfig{
		Name:          ConsumerActionsSchema,
		Description:   "Processes action execution schema definitions",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: "actions.schema.>",
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
