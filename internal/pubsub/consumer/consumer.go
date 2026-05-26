package consumer

import (
	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/actions"
	"github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/ratelimit"
	iredis "github.com/lunogram/platform/internal/redis"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/nats-io/nats.go/jetstream"

	"go.uber.org/zap"
)

const MsgIDHeader = "Nats-Msg-Id"

// Stream names for NATS JetStream.
const (
	StreamUsers              = "users"
	StreamUserEvents         = "users-events"
	StreamScheduled          = "scheduled"
	StreamLists              = "lists"
	StreamJourneys           = "journeys"
	StreamCampaigns          = "campaigns"
	StreamBroadcasts         = "broadcasts"
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

// Schedule names for automatically created schedules.
const (
	ScheduleAnniversary = "anniversary"
)

// Consumer names for NATS JetStream subscribers.
const (
	ConsumerUsersProcess              = "users-process"
	ConsumerUsersSchema               = "users-schema"
	ConsumerUserInboxProcess          = "users-inbox-process"
	ConsumerUserInboxDispatch         = "users-inbox-dispatch"
	ConsumerUserInboxRead             = "users-inbox-opened"
	ConsumerUserInboxArchived         = "users-inbox-archived"
	ConsumerUserInboxSent             = "users-inbox-sent"
	ConsumerUserEventsProcess         = "users-events-process"
	ConsumerUserEventsSchema          = "users-events-schema"
	ConsumerScheduledProcess          = "scheduled-process"
	ConsumerScheduledSchema           = "scheduled-schema"
	ConsumerListsRecompute            = "lists-recompute"
	ConsumerJourneysAdvance           = "journeys-advance"
	ConsumerJourneysAdvanceUser       = "journeys-advance-user"
	ConsumerJourneysEntrance          = "journeys-entrance"
	ConsumerCampaignsSend             = "campaigns-send"
	ConsumerBroadcastsProcess         = "broadcasts-process"
	ConsumerBroadcastsBatch           = "broadcasts-batch"
	ConsumerOrganizationsProcess      = "organizations-process"
	ConsumerOrganizationsSchema       = "organizations-schema"
	ConsumerOrganizationInboxProcess  = "organizations-inbox-process"
	ConsumerOrganizationInboxDispatch = "organizations-inbox-dispatch"
	ConsumerOrganizationInboxRead     = "organizations-inbox-opened"
	ConsumerOrganizationInboxArchived = "organizations-inbox-archived"
	ConsumerOrganizationInboxSent     = "organizations-inbox-sent"
	ConsumerOrganizationUsersProcess  = "organizations-users-process"
	ConsumerOrganizationUsersSchema   = "organizations-users-schema"
	ConsumerOrganizationEventsProcess = "organizations-events-process"
	ConsumerOrganizationEventsSchema  = "organizations-events-schema"
	ConsumerUserEventsMatch           = "users-events-match"
	ConsumerOrganizationEventsMatch   = "organizations-events-match"
	ConsumerActionsSchema             = "actions-schema"
	ConsumerScheduledBackfill         = "scheduled-backfill"
	ConsumerProjectEventsProcess      = "projects-events-process"
)

// Serve starts all JetStream consumers and registers their handlers.
//
// The recomputeLocker parameter deduplicates concurrent list recompute
// requests. When nil the handler processes every message unconditionally.
func Serve(ctx graceful.Context, jet jetstream.JetStream, logger *zap.Logger, ns Namespace, db *store.Connections, mgmt *management.State, usrs *subjects.State, jrny *journey.State, registry *providers.Registry, actionRegistry *actions.Registry, caller pubsub.Caller, limiter *ratelimit.Limiter, recomputeLocker *iredis.RecomputeLocker, schemaCache *iredis.SchemaCache, publicURL string, linkKey []byte, trackingURL string) {
	pub := pubsub.NewPublisher(jet, string(ns))
	renderer := pubsub.NewEmailRenderer(caller)
	router := NewRouter(ctx, jet, logger)

	router.HandleStream(ns.Stream(StreamUsers), ns.Consumer(ConsumerUsersProcess), UsersHandler(logger, usrs, pub, schemaCache))
	router.HandleStream(ns.Stream(StreamUsers), ns.Consumer(ConsumerUsersSchema), UserSchemasHandler(logger, usrs))
	userInbox := NewUserInboxHandler(logger, db.Subjects, mgmt, usrs, registry, pub, NewLimiter(limiter))
	router.HandleStream(ns.Stream(StreamUsers), ns.Consumer(ConsumerUserInboxProcess), userInbox.Messages())
	router.HandleStream(ns.Stream(StreamUsers), ns.Consumer(ConsumerUserInboxDispatch), userInbox.Dispatch())
	router.HandleStream(ns.Stream(StreamUsers), ns.Consumer(ConsumerUserInboxRead), userInbox.Read())
	router.HandleStream(ns.Stream(StreamUsers), ns.Consumer(ConsumerUserInboxArchived), userInbox.Archived())
	router.HandleStream(ns.Stream(StreamUsers), ns.Consumer(ConsumerUserInboxSent), userInbox.Sent())
	router.HandleStream(ns.Stream(StreamUserEvents), ns.Consumer(ConsumerUserEventsProcess), UserEventsHandler(logger, usrs, jrny, pub, schemaCache))
	router.HandleStream(ns.Stream(StreamUserEvents), ns.Consumer(ConsumerUserEventsSchema), UserEventSchemasHandler(logger, usrs))
	router.HandleStream(ns.Stream(StreamUserEvents), ns.Consumer(ConsumerUserEventsMatch), WithInProgress(MatchUserEventsHandler(logger, usrs, jrny, pub, schemaCache)))
	router.HandleStream(ns.Stream(StreamScheduled), ns.Consumer(ConsumerScheduledProcess), ScheduledHandler(logger, db.Subjects, usrs, pub, schemaCache))
	router.HandleStream(ns.Stream(StreamScheduled), ns.Consumer(ConsumerScheduledSchema), ScheduledSchemasHandler(logger, usrs))
	router.HandleStream(ns.Stream(StreamScheduled), ns.Consumer(ConsumerScheduledBackfill), ScheduledBackfillHandler(logger, usrs))
	router.HandleStream(ns.Stream(StreamLists), ns.Consumer(ConsumerListsRecompute), RecomputeListHandler(logger, usrs, pub, recomputeLocker))
	router.HandleStream(ns.Stream(StreamJourneys), ns.Consumer(ConsumerJourneysAdvance), JourneyStepHandler(logger, db.Subjects, jrny, mgmt, pub, actionRegistry, registry))
	router.HandleStream(ns.Stream(StreamJourneys), ns.Consumer(ConsumerJourneysEntrance), JourneyEntranceHandler(logger, jrny, pub))
	router.HandleStream(ns.Stream(StreamCampaigns), ns.Consumer(ConsumerCampaignsSend), CampaignsSendHandler(logger, db.Subjects, mgmt, usrs, renderer, pub, publicURL, linkKey, trackingURL))
	router.HandleStream(ns.Stream(StreamBroadcasts), ns.Consumer(ConsumerBroadcastsProcess), BroadcastProcessHandler(logger, mgmt, usrs, registry, pub, ns))
	router.HandleStream(ns.Stream(StreamBroadcasts), ns.Consumer(ConsumerBroadcastsBatch), BroadcastBatchHandler(logger, mgmt, usrs, pub, ns))
	router.HandleStream(ns.Stream(StreamOrganizations), ns.Consumer(ConsumerOrganizationsProcess), OrganizationsHandler(logger, usrs, pub, schemaCache))
	router.HandleStream(ns.Stream(StreamOrganizations), ns.Consumer(ConsumerOrganizationsSchema), OrganizationSchemasHandler(logger, usrs))
	orgInbox := NewOrganizationInboxHandler(logger, db.Subjects, mgmt, usrs, registry, pub, NewLimiter(limiter))
	router.HandleStream(ns.Stream(StreamOrganizations), ns.Consumer(ConsumerOrganizationInboxProcess), orgInbox.Messages())
	router.HandleStream(ns.Stream(StreamOrganizations), ns.Consumer(ConsumerOrganizationInboxDispatch), orgInbox.Dispatch())
	router.HandleStream(ns.Stream(StreamOrganizations), ns.Consumer(ConsumerOrganizationInboxRead), orgInbox.Read())
	router.HandleStream(ns.Stream(StreamOrganizations), ns.Consumer(ConsumerOrganizationInboxArchived), orgInbox.Archived())
	router.HandleStream(ns.Stream(StreamOrganizations), ns.Consumer(ConsumerOrganizationInboxSent), orgInbox.Sent())
	router.HandleStream(ns.Stream(StreamOrganizationUsers), ns.Consumer(ConsumerOrganizationUsersProcess), OrganizationUsersHandler(logger, usrs, pub, schemaCache))
	router.HandleStream(ns.Stream(StreamOrganizationUsers), ns.Consumer(ConsumerOrganizationUsersSchema), OrganizationUserSchemasHandler(logger, usrs))
	router.HandleStream(ns.Stream(StreamOrganizationEvents), ns.Consumer(ConsumerOrganizationEventsProcess), OrganizationEventsHandler(logger, usrs, jrny, pub, schemaCache))
	router.HandleStream(ns.Stream(StreamOrganizationEvents), ns.Consumer(ConsumerOrganizationEventsSchema), OrganizationEventSchemasHandler(logger, usrs))
	router.HandleStream(ns.Stream(StreamOrganizationEvents), ns.Consumer(ConsumerOrganizationEventsMatch), WithInProgress(MatchOrganizationEventsHandler(logger, usrs, jrny, pub, schemaCache)))
	router.HandleStream(ns.Stream(StreamActions), ns.Consumer(ConsumerActionsSchema), ActionSchemasHandler(logger, usrs))
	router.HandleCaller(ns.Subject(SubjectActionsExecute), ActionExecuteHandler(logger, actionRegistry, pub, schemaCache))
	router.HandleCaller(ns.Subject(SubjectActionsValidate), ActionValidateHandler(logger, actionRegistry))

}
