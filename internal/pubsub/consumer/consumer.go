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
	StreamUsers              = "users"
	StreamUserEvents         = "users-events"
	StreamLists              = "lists"
	StreamJourneys           = "journeys"
	StreamCampaigns          = "campaigns"
	StreamOrganizations      = "organizations"
	StreamOrganizationUsers  = "organizations-users"
	StreamOrganizationEvents = "organizations-events"
)

// Consumer names for NATS JetStream subscribers.
const (
	ConsumerUsersProcess              = "users-process"
	ConsumerUsersSchema               = "users-schema"
	ConsumerUserEventsProcess         = "users-events-process"
	ConsumerUserEventsSchema          = "users-events-schema"
	ConsumerListsRecompute            = "lists-recompute"
	ConsumerJourneysAdvance           = "journeys-advance"
	ConsumerCampaignsSend             = "campaigns-send"
	ConsumerOrganizationsProcess      = "organizations-process"
	ConsumerOrganizationsSchema       = "organizations-schema"
	ConsumerOrganizationUsersProcess  = "organizations-users-process"
	ConsumerOrganizationUsersSchema   = "organizations-users-schema"
	ConsumerOrganizationEventsProcess = "organizations-events-process"
	ConsumerOrganizationEventsSchema  = "organizations-events-schema"
)

// Serve starts all JetStream consumers and registers their handlers.
func Serve(ctx graceful.Context, jet jetstream.JetStream, logger *zap.Logger, db *store.Connections, mgmt *management.State, usrs *subjects.State, jrny *journey.State, registry *providers.Registry) {
	pub := pubsub.NewPublisher(jet)
	router := NewRouter(ctx, jet, logger)

	router.Handle(StreamUsers, ConsumerUsersProcess, UsersHandler(logger, usrs, pub))
	router.Handle(StreamUsers, ConsumerUsersSchema, UserSchemasHandler(logger, usrs))
	router.Handle(StreamUserEvents, ConsumerUserEventsProcess, UserEventsHandler(logger, usrs, jrny, pub))
	router.Handle(StreamUserEvents, ConsumerUserEventsSchema, UserEventSchemasHandler(logger, usrs))
	router.Handle(StreamLists, ConsumerListsRecompute, RecomputeListHandler(logger, usrs, pub))
	router.Handle(StreamJourneys, ConsumerJourneysAdvance, JourneyStepHandler(logger, db.Subjects, jrny, pub))
	router.Handle(StreamCampaigns, ConsumerCampaignsSend, CampaignsSendHandler(logger, mgmt, usrs, registry))
	router.Handle(StreamOrganizations, ConsumerOrganizationsProcess, OrganizationsHandler(logger, usrs, pub))
	router.Handle(StreamOrganizations, ConsumerOrganizationsSchema, OrganizationSchemasHandler(logger, usrs))
	router.Handle(StreamOrganizationUsers, ConsumerOrganizationUsersProcess, OrganizationUsersHandler(logger, usrs, pub))
	router.Handle(StreamOrganizationUsers, ConsumerOrganizationUsersSchema, OrganizationUserSchemasHandler(logger, usrs))
	router.Handle(StreamOrganizationEvents, ConsumerOrganizationEventsProcess, OrganizationEventsHandler(logger, usrs, jrny, pub))
	router.Handle(StreamOrganizationEvents, ConsumerOrganizationEventsSchema, OrganizationEventSchemasHandler(logger, usrs))
}
