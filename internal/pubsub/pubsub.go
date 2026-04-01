package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

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

// PublishOption configures optional Publish behaviour.
type PublishOption func(*publishOptions)

type publishOptions struct {
	// at schedules the message for future delivery. Zero means immediate.
	at time.Time
	// scheduleSubject overrides the subject the scheduled message is published
	// to. When empty, the publisher derives it automatically as
	// "schedules.<subject>".
	scheduleSubject string
	// msgID sets the Nats-Msg-Id header for JetStream publish
	// deduplication. When non-empty the server will silently discard a
	// message whose ID was already seen within the stream's
	// DuplicateWindow (default 2 minutes).
	msgID string
}

// At schedules the message for delivery at the given time.
// When the time is zero (the default) the message is delivered immediately.
// A non-zero value causes the publisher to set Nats-Schedule and
// Nats-Schedule-Target headers so that JetStream holds the message until the
// requested instant.
func At(t time.Time) PublishOption {
	return func(o *publishOptions) { o.at = t }
}

// WithScheduleSubject overrides the subject that the scheduled message is
// published to. By default the publisher prefixes the target subject with
// "schedules." which places it in the same stream that has
// AllowMsgSchedules enabled. Use this option only when the schedule subject
// deviates from that convention.
func WithScheduleSubject(subject string) PublishOption {
	return func(o *publishOptions) { o.scheduleSubject = subject }
}

// WithMsgID sets a Nats-Msg-Id header on the published message for
// JetStream server-side deduplication. If a message with the same ID
// was already stored within the stream's DuplicateWindow, the server
// silently discards the duplicate.
//
// Use this when the publisher might retry the same logical operation
// (e.g. a batch handler redelivered by NATS) to guarantee exactly-once
// publishing.
func WithMsgID(id string) PublishOption {
	return func(o *publishOptions) { o.msgID = id }
}

// Publisher publishes messages to JetStream subjects.
type Publisher interface {
	// Publish sends a message to the specified subject.
	//
	// The payload v can be any JSON-serialisable value or raw []byte.
	// When no options are provided the message is delivered immediately.
	// Pass At(t) to schedule the message for future delivery via the
	// Nats-Schedule mechanism.
	Publish(ctx context.Context, subject schemas.Subject, v any, opts ...PublishOption) error
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

func (p *publisher) Publish(ctx context.Context, subject schemas.Subject, v any, opts ...PublishOption) error {
	var o publishOptions
	for _, fn := range opts {
		fn(&o)
	}

	// Encode the payload. If the caller already provides raw bytes we use
	// them as-is; otherwise we JSON-marshal the value.
	var data []byte
	switch payload := v.(type) {
	case []byte:
		data = payload
	default:
		var err error
		data, err = json.Marshal(v)
		if err != nil {
			return fmt.Errorf("marshal publish payload: %w", err)
		}
	}

	subj := string(subject)
	if p.namespace != "" {
		subj = p.namespace + "." + subj
	}

	// Immediate publish – the common path.
	if o.at.IsZero() {
		var pubOpts []jetstream.PublishOpt
		if o.msgID != "" {
			pubOpts = append(pubOpts, jetstream.WithMsgID(o.msgID))
		}
		_, err := p.JetStream.Publish(ctx, subj, data, pubOpts...)
		return err
	}

	// Derive the schedule subject by inserting "schedules." before
	// the raw subject (after the namespace prefix, if any) so that
	// it lands in the same stream which must include
	// "schedules.<subject-prefix>.>" in its Subjects list.
	//
	// NATS requires one schedule per subject (ADR-51): "There may only
	// be one message per subject that holds a schedule." A unique ID is
	// appended so that concurrent delayed messages each get their own
	// slot instead of overwriting each other.
	//
	// Examples:
	//   no namespace: "campaigns.send.X.Y" → "schedules.campaigns.send.X.Y.<uuid>"
	//   namespace ns: "ns.campaigns.send.X.Y" → "ns.schedules.campaigns.send.X.Y.<uuid>"
	scheduleSubject := o.scheduleSubject
	if scheduleSubject == "" {
		scheduleSubject = "schedules." + string(subject) + "." + uuid.New().String()
	}

	// Ensure the namespace prefix is always present.
	if p.namespace != "" {
		scheduleSubject = p.namespace + "." + scheduleSubject
	}

	msg := nats.NewMsg(scheduleSubject)
	msg.Header.Set("Nats-Schedule", "@at "+o.at.UTC().Format(time.RFC3339))
	msg.Header.Set("Nats-Schedule-Target", subj)
	msg.Header.Set("Nats-Schedule-TTL", "24h")
	if o.msgID != "" {
		msg.Header.Set("Nats-Msg-Id", o.msgID)
	}
	msg.Data = data

	_, err := p.JetStream.PublishMsg(ctx, msg)
	return err
}

// Caller sends a message to a NATS subject and waits for a reply via the inbox pattern.
type Caller interface {
	// Call sends a JSON-encoded message and waits for a JSON response.
	// The caller is responsible for setting a deadline on the context.
	Call(ctx context.Context, subject schemas.Subject, v any) ([]byte, error)
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

func (r *caller) Call(ctx context.Context, subject schemas.Subject, v any) ([]byte, error) {
	payload, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

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

func (n *noopPublisher) Publish(_ context.Context, _ schemas.Subject, _ any, _ ...PublishOption) error {
	return nil
}

type noopCaller struct{}

// NewNoopCaller creates a Caller that does nothing for testing.
func NewNoopCaller() Caller {
	return &noopCaller{}
}

func (n *noopCaller) Call(_ context.Context, _ schemas.Subject, _ any) ([]byte, error) {
	return nil, nil
}
