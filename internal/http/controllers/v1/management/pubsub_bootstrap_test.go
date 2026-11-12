package v1

import (
	"sync"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/pubsub/consumer"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

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

	if err := consumer.Bootstrap(ctx, logger, jet, ""); err != nil {
		return err
	}

	defaultBootstrap.done = true
	return nil
}
