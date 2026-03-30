package redis

import (
	"errors"
	"fmt"

	"github.com/cloudproud/graceful"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// New creates a shared [redis.Client] from the given address and registers a
// graceful closer. Returns (nil, nil) when the address is empty, allowing
// callers that accept a nil client to fall back to their fail-open behaviour.
func New(ctx graceful.Context, logger *zap.Logger, address string) (*redis.Client, error) {
	if address == "" {
		logger.Warn("redis address not configured, features that depend on redis are disabled")
		return nil, nil
	}

	opts, err := redis.ParseURL(address)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %w", err)
	}

	client := redis.NewClient(opts)
	ctx.Closer(func() {
		if err := client.Close(); err != nil && !errors.Is(err, redis.ErrClosed) {
			logger.Error("failed to close redis client", zap.Error(err))
		}
	})

	return client, nil
}
