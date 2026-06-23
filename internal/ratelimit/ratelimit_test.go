package ratelimit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/container"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func newLimiter(t *testing.T) (*Limiter, context.Context) {
	t.Helper()

	logger := zaptest.NewLogger(t)
	connstr := container.RunRedis(t)

	opts, err := redis.ParseURL(connstr)
	require.NoError(t, err)

	client := redis.NewClient(opts)
	t.Cleanup(func() { client.Close() })

	prefix := fmt.Sprintf("test:%s:", uuid.New())
	limiter := New(client, prefix, logger)

	return limiter, t.Context()
}

func TestNilLimiterAllowsAll(t *testing.T) {
	t.Parallel()

	var limiter *Limiter
	allowed, retryAfter, err := limiter.Allow(context.Background(), "any-key", 1, time.Second)
	require.NoError(t, err)
	require.True(t, allowed)
	require.Zero(t, retryAfter)
}

func TestNilClientAllowsAll(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	limiter := New(nil, "prefix:", logger)

	allowed, retryAfter, err := limiter.Allow(context.Background(), "any-key", 1, time.Second)
	require.NoError(t, err)
	require.True(t, allowed)
	require.Zero(t, retryAfter)
}

func TestAllowSingleRequest(t *testing.T) {
	t.Parallel()

	limiter, ctx := newLimiter(t)

	allowed, retryAfter, err := limiter.Allow(ctx, "single", 5, time.Minute)
	require.NoError(t, err)
	require.True(t, allowed)
	require.Zero(t, retryAfter)
}

func TestAllowUpToLimit(t *testing.T) {
	t.Parallel()

	limiter, ctx := newLimiter(t)

	limit := 5
	key := "up-to-limit"

	for i := range limit {
		allowed, retryAfter, err := limiter.Allow(ctx, key, limit, time.Minute)
		require.NoError(t, err)
		require.True(t, allowed, "request %d should be allowed", i+1)
		require.Zero(t, retryAfter)
	}
}

func TestDenyOverLimit(t *testing.T) {
	t.Parallel()

	limiter, ctx := newLimiter(t)

	limit := 3
	key := "over-limit"

	for range limit {
		allowed, _, err := limiter.Allow(ctx, key, limit, time.Minute)
		require.NoError(t, err)
		require.True(t, allowed)
	}

	// The next request should be denied.
	allowed, retryAfter, err := limiter.Allow(ctx, key, limit, time.Minute)
	require.NoError(t, err)
	require.False(t, allowed)
	require.Greater(t, retryAfter, time.Duration(0))
}

func TestRetryAfterIsReasonable(t *testing.T) {
	t.Parallel()

	limiter, ctx := newLimiter(t)

	limit := 2
	window := 5 * time.Second
	key := "retry-after"

	for range limit {
		allowed, _, err := limiter.Allow(ctx, key, limit, window)
		require.NoError(t, err)
		require.True(t, allowed)
	}

	allowed, retryAfter, err := limiter.Allow(ctx, key, limit, window)
	require.NoError(t, err)
	require.False(t, allowed)
	require.Greater(t, retryAfter, time.Duration(0))
	require.LessOrEqual(t, retryAfter, window)
}

func TestSeparateKeysAreIndependent(t *testing.T) {
	t.Parallel()

	limiter, ctx := newLimiter(t)

	limit := 1

	allowed, _, err := limiter.Allow(ctx, "key-a", limit, time.Minute)
	require.NoError(t, err)
	require.True(t, allowed)

	// key-a is now exhausted.
	allowed, _, err = limiter.Allow(ctx, "key-a", limit, time.Minute)
	require.NoError(t, err)
	require.False(t, allowed)

	// key-b should still be allowed.
	allowed, _, err = limiter.Allow(ctx, "key-b", limit, time.Minute)
	require.NoError(t, err)
	require.True(t, allowed)
}

func TestWindowExpiry(t *testing.T) {
	t.Parallel()

	limiter, ctx := newLimiter(t)

	limit := 1
	window := 500 * time.Millisecond
	key := "window-expiry"

	allowed, _, err := limiter.Allow(ctx, key, limit, window)
	require.NoError(t, err)
	require.True(t, allowed)

	// Denied immediately.
	allowed, _, err = limiter.Allow(ctx, key, limit, window)
	require.NoError(t, err)
	require.False(t, allowed)

	// Wait for the window to expire.
	time.Sleep(window + 100*time.Millisecond)

	// Should be allowed again.
	allowed, _, err = limiter.Allow(ctx, key, limit, window)
	require.NoError(t, err)
	require.True(t, allowed)
}

func TestPrefixIsolation(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	connstr := container.RunRedis(t)

	opts, err := redis.ParseURL(connstr)
	require.NoError(t, err)

	client := redis.NewClient(opts)
	t.Cleanup(func() { client.Close() })

	limiterA := New(client, fmt.Sprintf("a:%s:", uuid.New()), logger)
	limiterB := New(client, fmt.Sprintf("b:%s:", uuid.New()), logger)

	limit := 1
	key := "same-key"

	allowed, _, err := limiterA.Allow(t.Context(), key, limit, time.Minute)
	require.NoError(t, err)
	require.True(t, allowed)

	// limiterA exhausted for this key.
	allowed, _, err = limiterA.Allow(t.Context(), key, limit, time.Minute)
	require.NoError(t, err)
	require.False(t, allowed)

	// limiterB with a different prefix should still allow.
	allowed, _, err = limiterB.Allow(t.Context(), key, limit, time.Minute)
	require.NoError(t, err)
	require.True(t, allowed)
}

func TestBurstThenRecover(t *testing.T) {
	t.Parallel()

	limiter, ctx := newLimiter(t)

	limit := 3
	window := 500 * time.Millisecond
	key := "burst-recover"

	// Exhaust the limit.
	for i := range limit {
		allowed, _, err := limiter.Allow(ctx, key, limit, window)
		require.NoError(t, err)
		require.True(t, allowed, "request %d should be allowed", i+1)
	}

	// All subsequent requests should be denied.
	for range 3 {
		allowed, _, err := limiter.Allow(ctx, key, limit, window)
		require.NoError(t, err)
		require.False(t, allowed)
	}

	// Wait for window to expire and verify full recovery.
	time.Sleep(window + 100*time.Millisecond)

	for i := range limit {
		allowed, _, err := limiter.Allow(ctx, key, limit, window)
		require.NoError(t, err)
		require.True(t, allowed, "request %d should be allowed after recovery", i+1)
	}
}

// --- Reserve tests ---

func TestNilLimiterReserveAllows(t *testing.T) {
	t.Parallel()

	var limiter *Limiter
	delay, err := limiter.Reserve(context.Background(), "any-key", 1, time.Second)
	require.NoError(t, err)
	require.Zero(t, delay)
}

func TestNilClientReserveAllows(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	limiter := New(nil, "prefix:", logger)

	delay, err := limiter.Reserve(context.Background(), "any-key", 1, time.Second)
	require.NoError(t, err)
	require.Zero(t, delay)
}

func TestReserveFirstRequestImmediate(t *testing.T) {
	t.Parallel()

	limiter, ctx := newLimiter(t)

	delay, err := limiter.Reserve(ctx, "reserve-first", 5, time.Minute)
	require.NoError(t, err)
	require.Zero(t, delay, "first request should have zero delay")
}

func TestReserveUpToLimitImmediate(t *testing.T) {
	t.Parallel()

	limiter, ctx := newLimiter(t)

	limit := 5
	key := "reserve-up-to"

	for i := range limit {
		delay, err := limiter.Reserve(ctx, key, limit, time.Minute)
		require.NoError(t, err)
		require.Zero(t, delay, "request %d should have zero delay", i+1)
	}
}

func TestReserveOverLimitReturnsIncreasingDelays(t *testing.T) {
	t.Parallel()

	limiter, ctx := newLimiter(t)

	limit := 2
	window := 10 * time.Second
	key := "reserve-over"

	// Fill the window.
	for range limit {
		delay, err := limiter.Reserve(ctx, key, limit, window)
		require.NoError(t, err)
		require.Zero(t, delay)
	}

	// Subsequent reservations should return increasing delays.
	var prevDelay time.Duration
	for i := range 5 {
		delay, err := limiter.Reserve(ctx, key, limit, window)
		require.NoError(t, err)
		require.Greater(t, delay, time.Duration(0), "reservation %d should have a positive delay", i+1)
		require.GreaterOrEqual(t, delay, prevDelay, "delays should be non-decreasing")
		prevDelay = delay
	}
}

func TestReserveSpacingMatchesRate(t *testing.T) {
	t.Parallel()

	limiter, ctx := newLimiter(t)

	limit := 1
	window := time.Second
	key := "reserve-spacing"

	// Reservations are scheduled relative to the first request's timestamp, so the
	// returned delay shrinks by however long the preceding calls took (Redis
	// round-trips, CI load). Comparing delay + elapsed-since-start against the
	// target keeps the assertion independent of that wall-clock jitter, which the
	// raw-delay check was sensitive to (and which accumulates across requests).
	start := time.Now()

	// First request: immediate.
	delay, err := limiter.Reserve(ctx, key, limit, window)
	require.NoError(t, err)
	require.Zero(t, delay)

	// Second request: its slot lands about one window after the first.
	delay, err = limiter.Reserve(ctx, key, limit, window)
	require.NoError(t, err)
	require.InDelta(t, window.Seconds(), delay.Seconds()+time.Since(start).Seconds(), 0.25,
		"second slot should land about one window after the first")

	// Third request: about two windows after the first.
	delay, err = limiter.Reserve(ctx, key, limit, window)
	require.NoError(t, err)
	require.InDelta(t, 2*window.Seconds(), delay.Seconds()+time.Since(start).Seconds(), 0.25,
		"third slot should land about two windows after the first")
}

func TestReserveSeparateKeysIndependent(t *testing.T) {
	t.Parallel()

	limiter, ctx := newLimiter(t)

	limit := 1
	window := time.Second

	// Exhaust key-a.
	delay, err := limiter.Reserve(ctx, "key-a", limit, window)
	require.NoError(t, err)
	require.Zero(t, delay)

	// key-a is full, next reservation has a delay.
	delay, err = limiter.Reserve(ctx, "key-a", limit, window)
	require.NoError(t, err)
	require.Greater(t, delay, time.Duration(0))

	// key-b should still be immediate.
	delay, err = limiter.Reserve(ctx, "key-b", limit, window)
	require.NoError(t, err)
	require.Zero(t, delay)
}
