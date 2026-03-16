package consumer

import (
	"slices"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/actions"
	"github.com/lunogram/platform/internal/config"
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
	StreamProjects           = "projects"
)

// Subscription subjects for NATS core subscribers.
const (
	SubjectActionsExecute  = "actions.execute.>"
	SubjectActionsValidate = "actions.validate.>"
)

// Consumer names for NATS JetStream subscribers.
const (
	ConsumerUsersProcess              = "users-process"
	ConsumerUsersSchema               = "users-schema"
	ConsumerUserEventsProcess         = "users-events-process"
	ConsumerUserEventsSchema          = "users-events-schema"
	ConsumerListsRecompute            = "lists-recompute"
	ConsumerJourneysAdvance           = "journeys-advance"
	ConsumerJourneysAdvanceUser       = "journeys-advance-user"
	ConsumerCampaignsSend             = "campaigns-send"
	ConsumerOrganizationsProcess      = "organizations-process"
	ConsumerOrganizationsSchema       = "organizations-schema"
	ConsumerOrganizationUsersProcess  = "organizations-users-process"
	ConsumerOrganizationUsersSchema   = "organizations-users-schema"
	ConsumerOrganizationEventsProcess = "organizations-events-process"
	ConsumerOrganizationEventsSchema  = "organizations-events-schema"
	ConsumerActionsSchema             = "actions-schema"
	ConsumerProjectEventsProcess      = "projects-events-process"
)

// Serve starts all JetStream consumers and registers their handlers.
func Serve(ctx graceful.Context, jet jetstream.JetStream, logger *zap.Logger, ns Namespace, db *store.Connections, mgmt *management.State, usrs *subjects.State, jrny *journey.State, registry *providers.Registry, actionRegistry *actions.Registry, caller pubsub.Caller, publicURL string, conf config.Node) {
	pub := pubsub.NewPublisher(jet, string(ns))
	renderer := pubsub.NewEmailRenderer(caller)
	router := NewRouter(ctx, jet, logger)

	if slices.Contains(conf.Modules, "consumers") {
		logger.Info("starting pub/sub consumers")

		router.HandleStream(ns.Stream(StreamUsers), ns.Consumer(ConsumerUsersProcess), UsersHandler(logger, usrs, pub))
		router.HandleStream(ns.Stream(StreamUsers), ns.Consumer(ConsumerUsersSchema), UserSchemasHandler(logger, usrs))
		router.HandleStream(ns.Stream(StreamUserEvents), ns.Consumer(ConsumerUserEventsProcess), UserEventsHandler(logger, usrs, jrny, pub))
		router.HandleStream(ns.Stream(StreamUserEvents), ns.Consumer(ConsumerUserEventsSchema), UserEventSchemasHandler(logger, usrs))
		router.HandleStream(ns.Stream(StreamLists), ns.Consumer(ConsumerListsRecompute), RecomputeListHandler(logger, usrs, pub))
		router.HandleStream(ns.Stream(StreamJourneys), ns.Consumer(ConsumerJourneysAdvance), JourneyStepHandler(logger, db.Subjects, jrny, mgmt, pub, actionRegistry))
		router.HandleStream(ns.Stream(StreamOrganizations), ns.Consumer(ConsumerOrganizationsProcess), OrganizationsHandler(logger, usrs, pub))
		router.HandleStream(ns.Stream(StreamOrganizations), ns.Consumer(ConsumerOrganizationsSchema), OrganizationSchemasHandler(logger, usrs))
		router.HandleStream(ns.Stream(StreamOrganizationUsers), ns.Consumer(ConsumerOrganizationUsersProcess), OrganizationUsersHandler(logger, usrs, pub))
		router.HandleStream(ns.Stream(StreamOrganizationUsers), ns.Consumer(ConsumerOrganizationUsersSchema), OrganizationUserSchemasHandler(logger, usrs))
		router.HandleStream(ns.Stream(StreamOrganizationEvents), ns.Consumer(ConsumerOrganizationEventsProcess), OrganizationEventsHandler(logger, usrs, jrny, pub))
		router.HandleStream(ns.Stream(StreamOrganizationEvents), ns.Consumer(ConsumerOrganizationEventsSchema), OrganizationEventSchemasHandler(logger, usrs))
	} else {
		logger.Info("skipping pub/sub consumers")
	}

	if slices.Contains(conf.Modules, "wasm") {
		logger.Info("starting wasm consumers")

		router.HandleStream(ns.Stream(StreamCampaigns), ns.Consumer(ConsumerCampaignsSend), CampaignsSendHandler(logger, mgmt, usrs, registry, renderer, publicURL))
		router.HandleStream(ns.Stream(StreamActions), ns.Consumer(ConsumerActionsSchema), ActionSchemasHandler(logger, usrs))
		router.HandleCaller(ns.Subject(SubjectActionsExecute), ActionExecuteHandler(logger, actionRegistry, pub))
		router.HandleCaller(ns.Subject(SubjectActionsValidate), ActionValidateHandler(logger, actionRegistry))
	} else {
		logger.Info("skipping wasm consumers")
	}
}
