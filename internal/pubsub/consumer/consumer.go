package consumer

import (
	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/actions"
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
	StreamActions            = "actions"
)

// Subscription subjects for NATS core subscribers.
const (
	SubjectActionsExecute = "actions.execute.>"
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
	ConsumerActionsSchema             = "actions-schema"
)

// Serve starts all JetStream consumers and registers their handlers.
func Serve(ctx graceful.Context, jet jetstream.JetStream, logger *zap.Logger, db *store.Connections, mgmt *management.State, usrs *subjects.State, jrny *journey.State, registry *providers.Registry, actionRegistry *actions.Registry) {
	pub := pubsub.NewPublisher(jet)
	router := NewRouter(ctx, jet, logger)

	router.HandleStream(StreamUsers, ConsumerUsersProcess, UsersHandler(logger, usrs, pub))
	router.HandleStream(StreamUsers, ConsumerUsersSchema, UserSchemasHandler(logger, usrs))
	router.HandleStream(StreamUserEvents, ConsumerUserEventsProcess, UserEventsHandler(logger, usrs, jrny, pub))
	router.HandleStream(StreamUserEvents, ConsumerUserEventsSchema, UserEventSchemasHandler(logger, usrs))
	router.HandleStream(StreamLists, ConsumerListsRecompute, RecomputeListHandler(logger, usrs, pub))
	router.HandleStream(StreamJourneys, ConsumerJourneysAdvance, JourneyStepHandler(logger, db.Subjects, jrny, mgmt, pub, actionRegistry))
	router.HandleStream(StreamCampaigns, ConsumerCampaignsSend, CampaignsSendHandler(logger, mgmt, usrs, registry))
	router.HandleStream(StreamOrganizations, ConsumerOrganizationsProcess, OrganizationsHandler(logger, usrs, pub))
	router.HandleStream(StreamOrganizations, ConsumerOrganizationsSchema, OrganizationSchemasHandler(logger, usrs))
	router.HandleStream(StreamOrganizationUsers, ConsumerOrganizationUsersProcess, OrganizationUsersHandler(logger, usrs, pub))
	router.HandleStream(StreamOrganizationUsers, ConsumerOrganizationUsersSchema, OrganizationUserSchemasHandler(logger, usrs))
	router.HandleStream(StreamOrganizationEvents, ConsumerOrganizationEventsProcess, OrganizationEventsHandler(logger, usrs, jrny, pub))
	router.HandleStream(StreamOrganizationEvents, ConsumerOrganizationEventsSchema, OrganizationEventSchemasHandler(logger, usrs))
	router.HandleStream(StreamActions, ConsumerActionsSchema, ActionSchemasHandler(logger, usrs))
	router.HandleCaller(SubjectActionsExecute, ActionExecuteHandler(logger, actionRegistry, pub))
}
