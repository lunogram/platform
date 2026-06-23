package redis

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/lunogram/platform/internal/container"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type record struct {
	Name string `json:"name"`
	N    int    `json:"n"`
}

func newTestCache(t *testing.T) *Cache[record] {
	t.Helper()
	opts, err := goredis.ParseURL(container.RunRedis(t))
	require.NoError(t, err)
	client := goredis.NewClient(opts)
	t.Cleanup(func() { _ = client.Close() })
	prefix := fmt.Sprintf("cachetest:%d:", time.Now().UnixNano())
	return NewCache[record](client, prefix, time.Hour, nil)
}

func TestCacheGetSetInvalidate(t *testing.T) {
	t.Parallel()
	c := newTestCache(t)
	ctx := context.Background()

	_, ok := c.Get(ctx, "a")
	assert.False(t, ok, "cold key misses")

	c.Set(ctx, "a", record{Name: "x", N: 1})
	got, ok := c.Get(ctx, "a")
	require.True(t, ok)
	assert.Equal(t, record{Name: "x", N: 1}, got)

	require.NoError(t, c.Invalidate(ctx, "a"))
	_, ok = c.Get(ctx, "a")
	assert.False(t, ok, "invalidated key misses")
}

func TestCacheGetOrLoadCoalesces(t *testing.T) {
	t.Parallel()
	c := newTestCache(t)
	ctx := context.Background()

	var calls int
	load := func(context.Context) (record, error) {
		calls++
		return record{Name: "loaded", N: 7}, nil
	}

	got, err := c.GetOrLoad(ctx, "k", load)
	require.NoError(t, err)
	assert.Equal(t, record{Name: "loaded", N: 7}, got)

	// Second call is served from the cache; load does not run again.
	got, err = c.GetOrLoad(ctx, "k", load)
	require.NoError(t, err)
	assert.Equal(t, record{Name: "loaded", N: 7}, got)
	assert.Equal(t, 1, calls, "a cached key must not re-run load")
}

func TestCacheGetOrLoadDoesNotCacheErrors(t *testing.T) {
	t.Parallel()
	c := newTestCache(t)
	ctx := context.Background()

	_, err := c.GetOrLoad(ctx, "k", func(context.Context) (record, error) {
		return record{}, errors.New("boom")
	})
	require.Error(t, err)

	got, err := c.GetOrLoad(ctx, "k", func(context.Context) (record, error) {
		return record{Name: "ok"}, nil
	})
	require.NoError(t, err)
	assert.Equal(t, "ok", got.Name, "a failed load must not be cached")
}

func TestCacheDisabledFallsThrough(t *testing.T) {
	t.Parallel()
	// A nil-client cache disables Redis: Get misses, GetOrLoad always loads.
	var c *Cache[record] = NewCache[record](nil, "x:", time.Hour, nil)
	ctx := context.Background()

	_, ok := c.Get(ctx, "k")
	assert.False(t, ok)

	var calls int
	for i := 0; i < 2; i++ {
		got, err := c.GetOrLoad(ctx, "k", func(context.Context) (record, error) {
			calls++
			return record{N: 1}, nil
		})
		require.NoError(t, err)
		assert.Equal(t, 1, got.N)
	}
	assert.Equal(t, 2, calls, "disabled cache loads every time")
	require.NoError(t, c.Invalidate(ctx, "k"))
}
