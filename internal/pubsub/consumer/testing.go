package consumer

import (
	"sync"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

type testBootstrap struct {
	once sync.Once
	err  error
}

// testBootstraps holds one testBootstrap per NATS server URL.
var testBootstraps sync.Map

// TestBootstrap ensures the default-namespace streams and consumers exist on
// the NATS server jet is connected to, once per test binary and server.
//
// Parallel tests share one NATS container. Bootstrapping per test makes them
// re-create the same streams and consumers concurrently, and under load a
// single JetStream call exceeds its default timeout.
func TestBootstrap(t *testing.T, ctx graceful.Context, logger *zap.Logger, jet jetstream.JetStream) {
	t.Helper()

	entry, _ := testBootstraps.LoadOrStore(jet.Conn().ConnectedUrl(), &testBootstrap{})
	bootstrap := entry.(*testBootstrap)
	bootstrap.once.Do(func() {
		bootstrap.err = Bootstrap(ctx, logger, jet, "")
	})

	if bootstrap.err != nil {
		t.Fatalf("consumer: failed to bootstrap streams and consumers: %v", bootstrap.err)
	}
}
