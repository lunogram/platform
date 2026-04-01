//go:build !enterprise

package consumer

import (
	"context"
	"fmt"

	internalProviders "github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// BroadcastProcessHandler returns a no-op handler in OSS builds.
func BroadcastProcessHandler(_ *zap.Logger, _ *management.State, _ *subjects.State, _ *internalProviders.Registry, _ pubsub.Publisher, _ Namespace) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		return Permanent(fmt.Errorf("broadcasts require enterprise"))
	}
}

// BroadcastBatchHandler returns a no-op handler in OSS builds.
func BroadcastBatchHandler(_ *zap.Logger, _ *management.State, _ *subjects.State, _ pubsub.Publisher, _ Namespace) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		return Permanent(fmt.Errorf("broadcasts require enterprise"))
	}
}
