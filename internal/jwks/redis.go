package jwks

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore is a Redis-backed [Store] for the shared L2 JWKS cache. A nil
// receiver or nil client is a no-op, so callers may pass it unconditionally.
type RedisStore struct {
	client *redis.Client
	prefix string
}

func NewRedisStore(client *redis.Client, prefix string) *RedisStore {
	return &RedisStore{client: client, prefix: prefix}
}

func (s *RedisStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if s == nil || s.client == nil {
		return nil, false, nil
	}
	v, err := s.client.Get(ctx, s.prefix+key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return v, true, nil
}

func (s *RedisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Set(ctx, s.prefix+key, value, ttl).Err()
}
