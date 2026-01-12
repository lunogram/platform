package consumer

import (
	"github.com/cloudproud/graceful"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/services/nexus/internal/pubsub"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// Stream names for NATS JetStream.
const (
	StreamUsers     = "users"
	StreamEvents    = "events"
	StreamRecompute = "recompute"
	StreamJourney   = "journey"
)

// Consumer names for NATS JetStream subscribers.
const (
	ConsumerUsers          = "users"
	ConsumerUserSchemas    = "user-schemas"
	ConsumerEvents         = "events"
	ConsumerEventSchemas   = "event-schemas"
	ConsumerJourneysState  = "journeys-state"
	ConsumerRecomputeLists = "recompute-lists"
)

// Serve starts all JetStream consumers and registers their handlers.
func Serve(ctx graceful.Context, jet jetstream.JetStream, logger *zap.Logger, db *sqlx.DB) {
	pub := pubsub.NewPublisher(jet)
	router := NewRouter(ctx, jet, logger)

	router.Handle(StreamUsers, ConsumerUsers, UsersHandler(logger, db, pub))
	router.Handle(StreamUsers, ConsumerUserSchemas, UserSchemasHandler(logger, db))
	router.Handle(StreamEvents, ConsumerEvents, EventsHandler(logger, db, pub))
	router.Handle(StreamEvents, ConsumerEventSchemas, EventSchemasHandler(logger, db))
	router.Handle(StreamRecompute, ConsumerRecomputeLists, RecomputeListHandler(logger, db, pub))
	router.Handle(StreamJourney, ConsumerJourneysState, JourneyStepHandler(logger, db, pub))
}
