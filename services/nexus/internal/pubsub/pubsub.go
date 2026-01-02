package pubsub

import (
	"context"
	"encoding/json"

	"github.com/cloudproud/graceful"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/nats-io/nats.go"
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

// HandlerFunc processes incoming messages from a JetStream consumer.
// If the handler returns an error, the message is negatively acknowledged (NAK)
// and will be redelivered. If the handler returns nil, the message is acknowledged (ACK)
// and removed from the stream.
type HandlerFunc func(ctx context.Context, msg jetstream.Msg) error

// New creates a new JetStream connection using the provided configuration.
func New(ctx graceful.Context, conf config.Node) (jetstream.JetStream, error) {
	conn, err := nats.Connect(conf.Nats.URL)
	if err != nil {
		return nil, err
	}

	ctx.Closer(conn.Close)

	jet, err := jetstream.New(conn)
	if err != nil {
		return nil, err
	}

	return jet, nil
}

// Serve starts all JetStream consumers and registers their handlers.
func Serve(ctx graceful.Context, jet jetstream.JetStream, logger *zap.Logger, db *sqlx.DB) {
	pub := NewPublisher(jet)
	router := NewRouter(ctx, jet, logger)

	router.Handle(StreamUsers, ConsumerUsers, UsersHandler(logger, db, pub))
	router.Handle(StreamUsers, ConsumerUserSchemas, UserSchemasHandler(logger, db))
	router.Handle(StreamEvents, ConsumerEvents, EventsHandler(logger, db, pub))
	router.Handle(StreamEvents, ConsumerEventSchemas, EventSchemasHandler(logger, db))
	router.Handle(StreamRecompute, ConsumerRecomputeLists, RecomputeListHandler(logger, db, pub))
	router.Handle(StreamJourney, ConsumerJourneysState, JourneyStepHandler(logger, db, pub))
}

// Subject represents a NATS subject for publishing messages.
type Subject string

// Publisher publishes messages to JetStream subjects.
type Publisher interface {
	// Publish sends a message to the specified subject with JSON-encoded payload.
	Publish(ctx context.Context, subject Subject, v any) error
}

type publisher struct {
	jetstream.JetStream
}

// NewPublisher creates a new Publisher that wraps the JetStream connection.
func NewPublisher(jet jetstream.JetStream) Publisher {
	return &publisher{JetStream: jet}
}

func (p *publisher) Publish(ctx context.Context, subject Subject, v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}

	_, err = p.JetStream.Publish(ctx, string(subject), payload)
	if err != nil {
		return err
	}

	return nil
}
