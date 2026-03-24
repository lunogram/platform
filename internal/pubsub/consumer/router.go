package consumer

import (
	"context"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/node/metrics"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// CallerHandlerFunc processes incoming messages from a NATS core subscription.
// This is used for request/reply patterns where JetStream consumers cannot be used.
type CallerHandlerFunc func(ctx context.Context, msg *nats.Msg)

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

// HandleStream registers a handler function for a specific stream and consumer.
// The consumer must already exist in JetStream configuration.
func (r *Router) HandleStream(stream, consumer string, handler HandlerFunc) {
	log := r.logger.With(zap.String("stream", stream), zap.String("consumer", consumer))
	log.Info("starting consumer")

	client, err := r.jet.Consumer(r.ctx, stream, consumer)
	if err != nil {
		log.Error("failed to get jetstream consumer, shutting down...", zap.Error(err))
		r.ctx.Shutdown()
		return
	}

	fn := func(msg jetstream.Msg) {
		start := time.Now()
		err := handler(r.ctx, msg)
		duration := time.Since(start).Seconds()
		metrics.NATSMessageProcessingDurationSeconds.WithLabelValues(stream, consumer).Observe(duration)

		if err != nil {
			if IsPermanent(err) {
				log.Warn("permanent error, terminating message", zap.Error(err))
				metrics.NATSMessagesTerminatedTotal.WithLabelValues(stream, consumer).Inc()
				if err := msg.Term(); err != nil {
					log.Error("failed to TERM message, shutting down...", zap.Error(err))
					r.ctx.Shutdown()
				}
				return
			}

			metrics.NATSMessagesNackedTotal.WithLabelValues(stream, consumer).Inc()
			if err := msg.Nak(); err != nil {
				log.Error("failed to NAK message, shutting down...", zap.Error(err))
				r.ctx.Shutdown()
			}

			return
		}

		err = msg.Ack()
		if err != nil {
			log.Error("failed to ACK message, shutting down...", zap.Error(err))
			r.ctx.Shutdown()
			return
		}

		metrics.NATSMessagesAckedTotal.WithLabelValues(stream, consumer).Inc()
	}

	_, err = client.Consume(fn)
	if err != nil {
		log.Error("failed to start jetstream consumer, shutting down...", zap.Error(err))
		r.ctx.Shutdown()
		return
	}
}

// HandleCaller registers a handler for a NATS core subscription.
// This is used for request/reply patterns where JetStream consumers cannot
// reply to NATS core requests.
func (r *Router) HandleCaller(subject string, handler CallerHandlerFunc) {
	log := r.logger.With(zap.String("subject", subject))
	log.Info("starting subscription")

	conn := r.jet.Conn()

	sub, err := conn.Subscribe(subject, func(msg *nats.Msg) {
		handler(r.ctx, msg)
	})
	if err != nil {
		log.Error("failed to subscribe, shutting down...", zap.Error(err))
		r.ctx.Shutdown()
		return
	}

	r.ctx.Closer(func() {
		_ = sub.Unsubscribe()
	})
}
