package redis

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

// Cache is a Redis-backed, JSON-serialised cache for a single value type, under
// a key prefix with a fixed TTL. It is concrete (no backend interface, since
// Redis is our only cache) and safe for concurrent use.
//
// A nil receiver or nil client disables caching: Get always misses, Set and
// Invalidate are no-ops, and GetOrLoad falls straight through to the loader.
// Every Redis error fails open — logged and treated as a miss — so an
// unavailable Redis degrades to the underlying source rather than an outage.
// Values are therefore a cache, never the source of truth; callers must tolerate
// a stale entry up to TTL (pair short TTLs with Invalidate on writes).
type Cache[T any] struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
	logger *zap.Logger
	group  singleflight.Group
}

// NewCache builds a cache writing keys as prefix+key with the given TTL. A nil
// client yields a disabled cache (pass-through), so callers can construct one
// unconditionally.
func NewCache[T any](client *redis.Client, prefix string, ttl time.Duration, logger *zap.Logger) *Cache[T] {
	return &Cache[T]{client: client, prefix: prefix, ttl: ttl, logger: logger}
}

func (c *Cache[T]) enabled() bool { return c != nil && c.client != nil }

// Get returns the cached value for key. ok is false on a miss, on a disabled
// cache, or on any Redis/decode error (fail-open).
func (c *Cache[T]) Get(ctx context.Context, key string) (value T, ok bool) {
	var zero T
	if !c.enabled() {
		return zero, false
	}
	b, err := c.client.Get(ctx, c.prefix+key).Bytes()
	if errors.Is(err, redis.Nil) {
		return zero, false
	}
	if err != nil {
		c.warn("cache get failed", key, err)
		return zero, false
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		c.warn("cache decode failed", key, err)
		return zero, false
	}
	return v, true
}

// Set stores value under key for the cache's TTL. It is best-effort: a disabled
// cache is a no-op and errors are logged, not returned.
func (c *Cache[T]) Set(ctx context.Context, key string, value T) {
	if !c.enabled() {
		return
	}
	b, err := json.Marshal(value)
	if err != nil {
		c.warn("cache encode failed", key, err)
		return
	}
	if err := c.client.Set(ctx, c.prefix+key, b, c.ttl).Err(); err != nil {
		c.warn("cache set failed", key, err)
	}
}

// GetOrLoad returns the cached value for key, or loads it via load on a miss,
// stores it, and returns it. Concurrent misses for the same key are coalesced so
// load runs once. A load error is returned and not cached.
func (c *Cache[T]) GetOrLoad(ctx context.Context, key string, load func(context.Context) (T, error)) (T, error) {
	if v, ok := c.Get(ctx, key); ok {
		return v, nil
	}
	if !c.enabled() {
		return load(ctx)
	}

	v, err, _ := c.group.Do(key, func() (any, error) {
		// Another caller may have populated the entry while we queued.
		if v, ok := c.Get(ctx, key); ok {
			return v, nil
		}
		v, err := load(ctx)
		if err != nil {
			return nil, err
		}
		c.Set(ctx, key, v)
		return v, nil
	})
	if err != nil {
		var zero T
		return zero, err
	}
	return v.(T), nil
}

// Invalidate removes key. It is best-effort on a disabled cache (no-op) and
// returns any Redis error so callers can decide whether to log or retry.
func (c *Cache[T]) Invalidate(ctx context.Context, key string) error {
	if !c.enabled() {
		return nil
	}
	return c.client.Del(ctx, c.prefix+key).Err()
}

func (c *Cache[T]) warn(msg, key string, err error) {
	if c.logger != nil {
		c.logger.Warn(msg, zap.String("prefix", c.prefix), zap.String("key", key), zap.Error(err))
	}
}
