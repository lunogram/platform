package consumer

import (
	"context"

	"github.com/cloudproud/graceful"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// HandlerFunc processes incoming messages from a JetStream consumer.
// If the handler returns an error, the message is negatively acknowledged (NAK)
// and will be redelivered. If the handler returns nil, the message is acknowledged (ACK)
// and removed from the stream.
type HandlerFunc func(ctx context.Context, msg jetstream.Msg) error

// NewRouter creates a new Router for registering JetStream consumer handlers.
func NewRouter(ctx graceful.Context, jet jetstream.JetStream, logger *zap.Logger) *Router {
	return &Router{
		ctx:    ctx,
		jet:    jet,
		logger: logger,
	}
}

// Router manages JetStream consumer subscriptions and message routing.
type Router struct {
	ctx    graceful.Context
	jet    jetstream.JetStream
	logger *zap.Logger
}

// Handle registers a handler function for a specific stream and consumer.
// The consumer must already exist in JetStream configuration.
func (r *Router) Handle(stream, consumer string, handler HandlerFunc) {
	log := r.logger.With(zap.String("stream", stream), zap.String("consumer", consumer))
	log.Info("starting consumer")

	client, err := r.jet.Consumer(r.ctx, stream, consumer)
	if err != nil {
		log.Error("failed to get jetstream consumer, shutting down...", zap.Error(err))
		r.ctx.Shutdown()
		return
	}

	fn := func(msg jetstream.Msg) {
		err := handler(r.ctx, msg)
		if err != nil {
			err = msg.Nak()
			if err != nil {
				log.Error("failed to NAK message, shutting down...", zap.Error(err))
				r.ctx.Shutdown()
				return
			}
		}

		err = msg.Ack()
		if err != nil {
			log.Error("failed to ACK message, shutting down...", zap.Error(err))
			r.ctx.Shutdown()
			return
		}
	}

	_, err = client.Consume(fn)
	if err != nil {
		log.Error("failed to start jetstream consumer, shutting down...", zap.Error(err))
		r.ctx.Shutdown()
		return
	}
}
