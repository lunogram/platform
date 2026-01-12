package pubsub

import (
	"context"
	"encoding/json"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/pubsub/schemas"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

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

// Publisher publishes messages to JetStream subjects.
type Publisher interface {
	// Publish sends a message to the specified subject with JSON-encoded payload.
	Publish(ctx context.Context, subject schemas.Subject, v any) error
}

type publisher struct {
	jetstream.JetStream
}

// NewPublisher creates a new Publisher that wraps the JetStream connection.
func NewPublisher(jet jetstream.JetStream) Publisher {
	return &publisher{JetStream: jet}
}

func (p *publisher) Publish(ctx context.Context, subject schemas.Subject, v any) error {
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
