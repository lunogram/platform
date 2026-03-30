package consumer

import (
	"context"

	"github.com/lunogram/platform/internal/ratelimit"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

const SchedulerHeader = "Nats-Scheduler"

func NewLimiter(limiter *ratelimit.Limiter) *Limiter {
	return &Limiter{Limiter: limiter}
}

type Limiter struct {
	*ratelimit.Limiter
}

// Throttle performs the rate-limit check for an incoming JetStream message.
//
// It follows the Reserve-then-schedule pattern:
//
//   - If the message was already delivered by the NATS scheduler (header
//     "Nats-Scheduler" present), the slot was previously reserved and the
//     message is allowed through (returns nil).
//   - If the limit is inactive, the message is allowed through (returns nil).
//   - If Reserve returns delay == 0, a slot is available now and the message
//     is allowed through (returns nil).
//   - If Reserve returns delay > 0, a [RateLimitedError] is returned. The
//     caller is responsible for republishing the message as a scheduled
//     delivery and then returning the error to the router so it can Ack
//     the original.
//
// Example usage:
//
//	if err := limiter.Throttle(ctx, logger, event.RateLimit, msg); err != nil {
//	    if rl, ok := IsRateLimited(err); ok {
//	        subject := schemas.CampaignsSend(event.ProjectID, event.CampaignID)
//	        if pubErr := pub.Publish(ctx, subject, event, pubsub.At(time.Now().Add(rl.RetryAfter))); pubErr != nil {
//	            return fmt.Errorf("schedule rate-limited message: %w", pubErr)
//	        }
//	    }
//	    return err
//	}
func (limiter *Limiter) Throttle(ctx context.Context, logger *zap.Logger, limit ratelimit.Limit, msg jetstream.Msg) error {
	if limiter == nil {
		return nil
	}

	// If the NATS scheduler delivered this message, the rate-limit slot was
	// already reserved by the original Reserve call — skip the limiter.
	if msg.Headers().Get(SchedulerHeader) != "" {
		return nil
	}

	if !limit.Active() {
		return nil
	}

	delay, _ := limiter.Reserve(ctx, limit.Key, limit.Requests, limit.Window)
	if delay == 0 {
		// Slot available now, caller may proceed.
		return nil
	}

	logger.Info("rate limited, scheduling redelivery",
		zap.Duration("delay", delay),
		zap.String("key", limit.Key),
		zap.Int("limit", limit.Requests),
		zap.Duration("window", limit.Window),
	)

	return RateLimited(delay)
}
