package consumer

import (
	"context"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

var (
	// DefaultBackOff defines the retry backoff durations for consumers.
	DefaultBackOff = []time.Duration{1 * time.Second, 5 * time.Second, 30 * time.Second, 2 * time.Minute, 10 * time.Minute}

	// DefaultMaxDeliver is the maximum number of delivery attempts for a message.
	// Must be greater than len(DefaultBackOff) per NATS requirements.
	DefaultMaxDeliver = len(DefaultBackOff) + 1

	// ProcessMaxDeliver is the maximum number of delivery attempts for process consumers.
	// After exhausting the BackOff schedule, remaining retries use the last backoff interval (10m).
	// With 20 attempts: 5 backoff steps + 14 retries at 10m = ~2h 23m + 13s of total retry time.
	ProcessMaxDeliver = 20
)

func Bootstrap(ctx graceful.Context, logger *zap.Logger, jet jetstream.JetStream, ns Namespace) error {
	logger.Info("bootstrapping pubsub streams and consumers...", zap.String("namespace", string(ns)))
	bootstrap := NewBootstrapper(logger, jet)

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        ns.Stream(StreamUsers),
		Description: "Responsible for receiving incoming users",
		Subjects:    []string{ns.Subject("users.process.>"), ns.Subject("users.schema.>")},
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})

	bootstrap.EnsureConsumer(ctx, ns.Stream(StreamUsers), jetstream.ConsumerConfig{
		Name:          ns.Consumer(ConsumerUsersProcess),
		FilterSubject: ns.Subject("users.process.>"),
		Description:   "Processes incoming users",
		AckPolicy:     jetstream.AckExplicitPolicy,
		BackOff:       DefaultBackOff,
		MaxDeliver:    ProcessMaxDeliver,
	})

	bootstrap.EnsureConsumer(ctx, ns.Stream(StreamUsers), jetstream.ConsumerConfig{
		Name:          ns.Consumer(ConsumerUsersSchema),
		FilterSubject: ns.Subject("users.schema.>"),
		Description:   "Processes user schema definitions",
		AckPolicy:     jetstream.AckExplicitPolicy,
		BackOff:       DefaultBackOff,
		MaxDeliver:    DefaultMaxDeliver,
	})

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        ns.Stream(StreamUserEvents),
		Description: "Responsible for receiving incoming user events",
		Subjects:    []string{ns.Subject("users.events.>")},
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})

	bootstrap.EnsureConsumer(ctx, ns.Stream(StreamUserEvents), jetstream.ConsumerConfig{
		Name:          ns.Consumer(ConsumerUserEventsProcess),
		FilterSubject: ns.Subject("users.events.process.>"),
		Description:   "Processes incoming user events",
		AckPolicy:     jetstream.AckExplicitPolicy,
		BackOff:       DefaultBackOff,
		MaxDeliver:    ProcessMaxDeliver,
	})

	bootstrap.EnsureConsumer(ctx, ns.Stream(StreamUserEvents), jetstream.ConsumerConfig{
		Name:          ns.Consumer(ConsumerUserEventsSchema),
		FilterSubject: ns.Subject("users.events.schema.>"),
		Description:   "Processes user event schema definitions",
		AckPolicy:     jetstream.AckExplicitPolicy,
		BackOff:       DefaultBackOff,
		MaxDeliver:    DefaultMaxDeliver,
	})

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        ns.Stream(StreamScheduled),
		Description: "Responsible for receiving incoming scheduled entities (user and organization)",
		Subjects:    []string{ns.Subject("scheduled.process.>"), ns.Subject("scheduled.schema.>"), ns.Subject("scheduled.backfill.>")},
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})

	bootstrap.EnsureConsumer(ctx, ns.Stream(StreamScheduled), jetstream.ConsumerConfig{
		Name:          ns.Consumer(ConsumerScheduledProcess),
		FilterSubject: ns.Subject("scheduled.process.>"),
		Description:   "Processes incoming scheduled entities",
		AckPolicy:     jetstream.AckExplicitPolicy,
		BackOff:       DefaultBackOff,
		MaxDeliver:    ProcessMaxDeliver,
	})

	bootstrap.EnsureConsumer(ctx, ns.Stream(StreamScheduled), jetstream.ConsumerConfig{
		Name:          ns.Consumer(ConsumerScheduledSchema),
		FilterSubject: ns.Subject("scheduled.schema.>"),
		Description:   "Processes scheduled schema definitions",
		AckPolicy:     jetstream.AckExplicitPolicy,
		BackOff:       DefaultBackOff,
		MaxDeliver:    DefaultMaxDeliver,
	})

	bootstrap.EnsureConsumer(ctx, ns.Stream(StreamScheduled), jetstream.ConsumerConfig{
		Name:          ns.Consumer(ConsumerScheduledBackfill),
		FilterSubject: ns.Subject("scheduled.backfill.>"),
		Description:   "Backfills scheduled events when a new offset is created",
		AckPolicy:     jetstream.AckExplicitPolicy,
		BackOff:       DefaultBackOff,
		MaxDeliver:    ProcessMaxDeliver,
	})

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        ns.Stream(StreamLists),
		Description: "List recomputation triggers",
		Subjects:    []string{ns.Subject("lists.>")},
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})

	bootstrap.EnsureConsumer(ctx, ns.Stream(StreamLists), jetstream.ConsumerConfig{
		Name:          ns.Consumer(ConsumerListsRecompute),
		Description:   "Processes list recomputation requests",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: ns.Subject("lists.recompute.>"),
		BackOff:       DefaultBackOff,
		MaxDeliver:    ProcessMaxDeliver,
	})

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        ns.Stream(StreamJourneys),
		Description: "Journey advancement and orchestration",
		Subjects:    []string{ns.Subject("journeys.>")},
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})

	bootstrap.EnsureConsumer(ctx, ns.Stream(StreamJourneys), jetstream.ConsumerConfig{
		Name:          ns.Consumer(ConsumerJourneysAdvance),
		Description:   "Processes journey advancement requests",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: ns.Subject("journeys.advance.>"),
		BackOff:       DefaultBackOff,
		MaxDeliver:    ProcessMaxDeliver,
	})

	bootstrap.EnsureConsumer(ctx, ns.Stream(StreamJourneys), jetstream.ConsumerConfig{
		Name:          ns.Consumer(ConsumerJourneysAdvanceUser),
		Description:   "Processes journey advancement requests per specific user",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: ns.Subject("journeys.advance.>"),
		BackOff:       DefaultBackOff,
		MaxDeliver:    ProcessMaxDeliver,
	})

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        ns.Stream(StreamCampaigns),
		Description: "Campaign sending and execution",
		Subjects:    []string{ns.Subject("campaigns.>")},
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})

	bootstrap.EnsureConsumer(ctx, ns.Stream(StreamCampaigns), jetstream.ConsumerConfig{
		Name:          ns.Consumer(ConsumerCampaignsSend),
		Description:   "Processes campaign send requests",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: ns.Subject("campaigns.send.>"),
		BackOff:       DefaultBackOff,
		MaxDeliver:    ProcessMaxDeliver,
	})

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        ns.Stream(StreamBroadcasts),
		Description: "Broadcast processing and execution",
		Subjects:    []string{ns.Subject("broadcasts.process.>"), ns.Subject("broadcasts.batch.>")},
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})

	bootstrap.EnsureConsumer(ctx, ns.Stream(StreamBroadcasts), jetstream.ConsumerConfig{
		Name:          ns.Consumer(ConsumerBroadcastsProcess),
		Description:   "Processes broadcast send requests",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: ns.Subject("broadcasts.process.>"),
		BackOff:       DefaultBackOff,
		MaxDeliver:    ProcessMaxDeliver,
	})

	bootstrap.EnsureConsumer(ctx, ns.Stream(StreamBroadcasts), jetstream.ConsumerConfig{
		Name:          ns.Consumer(ConsumerBroadcastsBatch),
		Description:   "Processes broadcast batch fan-out",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: ns.Subject("broadcasts.batch.>"),
		BackOff:       DefaultBackOff,
		MaxDeliver:    ProcessMaxDeliver,
	})

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        ns.Stream(StreamOrganizations),
		Description: "Organization processing and schema extraction",
		Subjects:    []string{ns.Subject("organizations.process.>"), ns.Subject("organizations.schema.>")},
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})

	bootstrap.EnsureConsumer(ctx, ns.Stream(StreamOrganizations), jetstream.ConsumerConfig{
		Name:          ns.Consumer(ConsumerOrganizationsProcess),
		Description:   "Processes incoming organizations",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: ns.Subject("organizations.process.>"),
		BackOff:       DefaultBackOff,
		MaxDeliver:    ProcessMaxDeliver,
	})

	bootstrap.EnsureConsumer(ctx, ns.Stream(StreamOrganizations), jetstream.ConsumerConfig{
		Name:          ns.Consumer(ConsumerOrganizationsSchema),
		Description:   "Processes organization schema definitions",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: ns.Subject("organizations.schema.>"),
		BackOff:       DefaultBackOff,
		MaxDeliver:    DefaultMaxDeliver,
	})

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        ns.Stream(StreamOrganizationUsers),
		Description: "Organization user membership processing",
		Subjects:    []string{ns.Subject("organizations.users.>")},
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})

	bootstrap.EnsureConsumer(ctx, ns.Stream(StreamOrganizationUsers), jetstream.ConsumerConfig{
		Name:          ns.Consumer(ConsumerOrganizationUsersProcess),
		Description:   "Processes organization user memberships",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: ns.Subject("organizations.users.process.>"),
		BackOff:       DefaultBackOff,
		MaxDeliver:    ProcessMaxDeliver,
	})

	bootstrap.EnsureConsumer(ctx, ns.Stream(StreamOrganizationUsers), jetstream.ConsumerConfig{
		Name:          ns.Consumer(ConsumerOrganizationUsersSchema),
		Description:   "Processes organization user schema definitions",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: ns.Subject("organizations.users.schema.>"),
		BackOff:       DefaultBackOff,
		MaxDeliver:    DefaultMaxDeliver,
	})

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        ns.Stream(StreamOrganizationEvents),
		Description: "Organization event processing",
		Subjects:    []string{ns.Subject("organizations.events.>")},
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})

	bootstrap.EnsureConsumer(ctx, ns.Stream(StreamOrganizationEvents), jetstream.ConsumerConfig{
		Name:          ns.Consumer(ConsumerOrganizationEventsProcess),
		Description:   "Processes incoming organization events",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: ns.Subject("organizations.events.process.>"),
		BackOff:       DefaultBackOff,
		MaxDeliver:    ProcessMaxDeliver,
	})

	bootstrap.EnsureConsumer(ctx, ns.Stream(StreamOrganizationEvents), jetstream.ConsumerConfig{
		Name:          ns.Consumer(ConsumerOrganizationEventsSchema),
		FilterSubject: ns.Subject("organizations.events.schema.>"),
		Description:   "Processes organization event schema definitions",
		AckPolicy:     jetstream.AckExplicitPolicy,
		BackOff:       DefaultBackOff,
		MaxDeliver:    DefaultMaxDeliver,
	})

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        ns.Stream(StreamActions),
		Description: "Action schema extraction",
		Subjects:    []string{ns.Subject("actions.schema.>")},
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})

	bootstrap.EnsureConsumer(ctx, ns.Stream(StreamActions), jetstream.ConsumerConfig{
		Name:          ns.Consumer(ConsumerActionsSchema),
		Description:   "Processes action execution schema definitions",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: ns.Subject("actions.schema.>"),
		BackOff:       DefaultBackOff,
		MaxDeliver:    DefaultMaxDeliver,
	})

	bootstrap.EnsureStream(ctx, jetstream.StreamConfig{
		Name:        ns.Stream(StreamProjects),
		Description: "Project event processing",
		Subjects:    []string{ns.Subject("projects.events.>")},
		Discard:     jetstream.DiscardOld,
		MaxAge:      24 * time.Hour,
		Replicas:    1,
	})

	bootstrap.EnsureConsumer(ctx, ns.Stream(StreamProjects), jetstream.ConsumerConfig{
		Name:          ns.Consumer(ConsumerProjectEventsProcess),
		Description:   "Processes incoming project events",
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: ns.Subject("projects.events.>"),
		BackOff:       DefaultBackOff,
		MaxDeliver:    ProcessMaxDeliver,
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
