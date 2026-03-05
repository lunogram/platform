package pubsub

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// New creates a new JetStream connection using the provided configuration.
// Use jet.Conn() to access the underlying NATS connection for request/reply patterns.
func New(ctx graceful.Context, conf config.Node) (jetstream.JetStream, error) {
	conn, err := nats.Connect(
		conf.Nats.URL,
		nats.MaxReconnects(5),
		nats.ReconnectWait(100*time.Millisecond),
		nats.RetryOnFailedConnect(true),
	)
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
	namespace string
}

// NewPublisher creates a new Publisher that wraps the JetStream connection.
// When the namespace is non-empty every subject is prefixed with it.
func NewPublisher(jet jetstream.JetStream, namespace string) Publisher {
	return &publisher{JetStream: jet, namespace: namespace}
}

func (p *publisher) Publish(ctx context.Context, subject schemas.Subject, v any) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return err
	}

	subj := string(subject)
	if p.namespace != "" {
		subj = p.namespace + "." + subj
	}

	_, err = p.JetStream.Publish(ctx, subj, payload)
	if err != nil {
		return err
	}

	return nil
}

// Caller sends a message to a NATS subject and waits for a reply via the inbox pattern.
type Caller interface {
	// Call sends a JSON-encoded message and waits for a JSON response.
	Call(ctx context.Context, subject schemas.Subject, v any, timeout time.Duration) ([]byte, error)
}

type caller struct {
	conn      *nats.Conn
	namespace string
}

// NewCaller creates a Caller that uses the underlying NATS connection for request/reply.
// When the namespace is non-empty every subject is prefixed with it.
func NewCaller(jet jetstream.JetStream, namespace string) Caller {
	return &caller{conn: jet.Conn(), namespace: namespace}
}

func (r *caller) Call(ctx context.Context, subject schemas.Subject, v any, timeout time.Duration) ([]byte, error) {
	payload, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	subj := string(subject)
	if r.namespace != "" {
		subj = r.namespace + "." + subj
	}

	msg, err := r.conn.RequestWithContext(ctx, subj, payload)
	if err != nil {
		return nil, err
	}

	return msg.Data, nil
}

type noopPublisher struct{}

// NewNoopPublisher creates a Publisher that does nothing for testing.
func NewNoopPublisher() Publisher {
	return &noopPublisher{}
}

func (n *noopPublisher) Publish(ctx context.Context, subject schemas.Subject, v any) error {
	return nil
}

type noopCaller struct{}

// NewNoopCaller creates a Caller that does nothing for testing.
func NewNoopCaller() Caller {
	return &noopCaller{}
}

func (n *noopCaller) Call(ctx context.Context, subject schemas.Subject, v any, timeout time.Duration) ([]byte, error) {
	return nil, nil
}
