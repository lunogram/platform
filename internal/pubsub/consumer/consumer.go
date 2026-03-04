package consumer

import (
	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// Stream names for NATS JetStream.
const (
	StreamUsers     = "users"
	StreamEvents    = "events"
	StreamLists     = "lists"
	StreamJourneys  = "journeys"
	StreamCampaigns = "campaigns"
)

// Consumer names for NATS JetStream subscribers.
const (
	ConsumerUsersProcess    = "users-process"
	ConsumerUsersSchema     = "users-schema"
	ConsumerEventsProcess   = "events-process"
	ConsumerEventsSchema    = "events-schema"
	ConsumerListsRecompute  = "lists-recompute"
	ConsumerJourneysAdvance = "journeys-advance"
	ConsumerCampaignsSend   = "campaigns-send"
)

// Serve starts all JetStream consumers and registers their handlers.
func Serve(ctx graceful.Context, jet jetstream.JetStream, logger *zap.Logger, db *store.Connections, mgmt *management.State, usrs *subjects.State, jrny *journey.State, registry *providers.Registry) {
	pub := pubsub.NewPublisher(jet)
	router := NewRouter(ctx, jet, logger)

	router.Handle(StreamUsers, ConsumerUsersProcess, UsersHandler(logger, usrs, pub))
	router.Handle(StreamUsers, ConsumerUsersSchema, UserSchemasHandler(logger, usrs))
	router.Handle(StreamEvents, ConsumerEventsProcess, EventsHandler(logger, usrs, jrny, pub))
	router.Handle(StreamEvents, ConsumerEventsSchema, EventSchemasHandler(logger, usrs))
	router.Handle(StreamLists, ConsumerListsRecompute, RecomputeListHandler(logger, usrs, pub))
	router.Handle(StreamJourneys, ConsumerJourneysAdvance, JourneyStepHandler(logger, db.Subjects, jrny, pub))
	router.Handle(StreamCampaigns, ConsumerCampaignsSend, CampaignsSendHandler(logger, mgmt, usrs, registry))
}
