package v1

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/pubsub/consumer"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// defaultBootstrapTimeout bounds a single bootstrap attempt. Every parallel
// test in this package waits on the same lock, so an attempt without a
// deadline would stall the whole package silently instead of failing.
const defaultBootstrapTimeout = 60 * time.Second

var defaultBootstrap struct {
	sync.Mutex
	done bool
}

// ensureDefaultPubsubBootstrapped bootstraps the default namespace once per
// test process for this package. Management controller tests share the same
// reused NATS container and namespace, so repeated parallel bootstraps contend
// on the same stream/consumer updates and occasionally time out.
func ensureDefaultPubsubBootstrapped(ctx graceful.Context, logger *zap.Logger, jet jetstream.JetStream) error {
	defaultBootstrap.Lock()
	defer defaultBootstrap.Unlock()

	if defaultBootstrap.done {
		return nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, defaultBootstrapTimeout)
	defer cancel()

	if err := consumer.Bootstrap(graceful.NewContext(timeoutCtx), logger, jet, ""); err != nil {
		return fmt.Errorf("bootstrap default pubsub namespace: %w", err)
	}

	if err := timeoutCtx.Err(); err != nil {
		return fmt.Errorf("bootstrap default pubsub namespace: %w", err)
	}

	defaultBootstrap.done = true
	return nil
}
