package redis

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/container"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRedisClient(t *testing.T) *redis.Client {
	t.Helper()

	connstr := container.RunRedis(t)
	opts, err := redis.ParseURL(connstr)
	require.NoError(t, err)

	client := redis.NewClient(opts)
	t.Cleanup(func() { client.Close() })

	return client
}

func testPrefix(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("test:%s:", uuid.New())
}

func TestNewRecomputeLocker(t *testing.T) {
	t.Parallel()

	t.Run("with redis client", func(t *testing.T) {
		t.Parallel()
		client := newTestRedisClient(t)
		locker := NewRecomputeLocker(client, "prefix:")
		assert.NotNil(t, locker)
		assert.Equal(t, client, locker.redis)
		assert.Equal(t, "prefix:", locker.prefix)
	})

	t.Run("with nil client", func(t *testing.T) {
		t.Parallel()
		locker := NewRecomputeLocker(nil, "")
		assert.NotNil(t, locker)
		assert.Nil(t, locker.redis)
	})
}

func TestRecomputeLockerNilSafety(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	listID := uuid.New()
	ctx := context.Background()

	t.Run("nil locker acquire returns true", func(t *testing.T) {
		t.Parallel()
		var locker *RecomputeLocker
		acquired, gen := locker.Acquire(ctx, projectID, listID)
		assert.True(t, acquired)
		assert.Equal(t, int64(0), gen)
	})

	t.Run("nil locker generation returns zero", func(t *testing.T) {
		t.Parallel()
		var locker *RecomputeLocker
		gen := locker.Generation(ctx, projectID, listID)
		assert.Equal(t, int64(0), gen)
	})

	t.Run("nil locker release does not panic", func(t *testing.T) {
		t.Parallel()
		var locker *RecomputeLocker
		assert.NotPanics(t, func() {
			locker.Release(ctx, projectID, listID)
		})
	})

	t.Run("nil redis client acquire returns true", func(t *testing.T) {
		t.Parallel()
		locker := NewRecomputeLocker(nil, "")
		acquired, gen := locker.Acquire(ctx, projectID, listID)
		assert.True(t, acquired)
		assert.Equal(t, int64(0), gen)
	})

	t.Run("nil redis client generation returns zero", func(t *testing.T) {
		t.Parallel()
		locker := NewRecomputeLocker(nil, "")
		gen := locker.Generation(ctx, projectID, listID)
		assert.Equal(t, int64(0), gen)
	})

	t.Run("nil redis client release does not panic", func(t *testing.T) {
		t.Parallel()
		locker := NewRecomputeLocker(nil, "")
		assert.NotPanics(t, func() {
			locker.Release(ctx, projectID, listID)
		})
	})
}

func TestRecomputeLockerAcquireSingle(t *testing.T) {
	t.Parallel()

	client := newTestRedisClient(t)
	prefix := testPrefix(t)
	locker := NewRecomputeLocker(client, prefix)
	ctx := context.Background()

	projectID := uuid.New()
	listID := uuid.New()

	acquired, gen := locker.Acquire(ctx, projectID, listID)
	assert.True(t, acquired, "first acquire should succeed")
	assert.Equal(t, int64(1), gen, "first generation should be 1")
}

func TestRecomputeLockerAcquireSecondBlocked(t *testing.T) {
	t.Parallel()

	client := newTestRedisClient(t)
	prefix := testPrefix(t)
	locker := NewRecomputeLocker(client, prefix)
	ctx := context.Background()

	projectID := uuid.New()
	listID := uuid.New()

	acquired1, gen1 := locker.Acquire(ctx, projectID, listID)
	assert.True(t, acquired1)
	assert.Equal(t, int64(1), gen1)

	// Second acquire for the same list should fail (active lock held).
	acquired2, gen2 := locker.Acquire(ctx, projectID, listID)
	assert.False(t, acquired2, "second acquire should be blocked")
	assert.Equal(t, int64(2), gen2, "generation should still increment")
}

func TestRecomputeLockerAcquireDifferentLists(t *testing.T) {
	t.Parallel()

	client := newTestRedisClient(t)
	prefix := testPrefix(t)
	locker := NewRecomputeLocker(client, prefix)
	ctx := context.Background()

	projectID := uuid.New()
	listA := uuid.New()
	listB := uuid.New()

	acquiredA, _ := locker.Acquire(ctx, projectID, listA)
	acquiredB, _ := locker.Acquire(ctx, projectID, listB)

	assert.True(t, acquiredA, "list A should be acquired")
	assert.True(t, acquiredB, "list B should be acquired independently")
}

func TestRecomputeLockerAcquireDifferentProjects(t *testing.T) {
	t.Parallel()

	client := newTestRedisClient(t)
	prefix := testPrefix(t)
	locker := NewRecomputeLocker(client, prefix)
	ctx := context.Background()

	projectA := uuid.New()
	projectB := uuid.New()
	listID := uuid.New()

	acquiredA, _ := locker.Acquire(ctx, projectA, listID)
	acquiredB, _ := locker.Acquire(ctx, projectB, listID)

	assert.True(t, acquiredA, "project A should be acquired")
	assert.True(t, acquiredB, "project B should be acquired independently")
}

func TestRecomputeLockerPrefixIsolation(t *testing.T) {
	t.Parallel()

	client := newTestRedisClient(t)
	ctx := context.Background()

	projectID := uuid.New()
	listID := uuid.New()

	lockerA := NewRecomputeLocker(client, "prefixA:")
	lockerB := NewRecomputeLocker(client, "prefixB:")

	acquiredA, _ := lockerA.Acquire(ctx, projectID, listID)
	acquiredB, _ := lockerB.Acquire(ctx, projectID, listID)

	assert.True(t, acquiredA)
	assert.True(t, acquiredB, "different prefixes should not interfere")
}

func TestRecomputeLockerGenerationStableNoReloop(t *testing.T) {
	t.Parallel()

	client := newTestRedisClient(t)
	prefix := testPrefix(t)
	locker := NewRecomputeLocker(client, prefix)
	ctx := context.Background()

	projectID := uuid.New()
	listID := uuid.New()

	acquired, gen := locker.Acquire(ctx, projectID, listID)
	require.True(t, acquired)

	// No other messages arrived, so generation hasn't advanced.
	latest := locker.Generation(ctx, projectID, listID)
	assert.Equal(t, gen, latest, "generation should match when no new messages arrived")

	// Release the lock.
	locker.Release(ctx, projectID, listID)

	// Active lock should be released — a new acquire should succeed.
	acquired2, _ := locker.Acquire(ctx, projectID, listID)
	assert.True(t, acquired2, "lock should be available after release")
}

func TestRecomputeLockerGenerationAdvancedReloop(t *testing.T) {
	t.Parallel()

	client := newTestRedisClient(t)
	prefix := testPrefix(t)
	locker := NewRecomputeLocker(client, prefix)
	ctx := context.Background()

	projectID := uuid.New()
	listID := uuid.New()

	acquired, gen := locker.Acquire(ctx, projectID, listID)
	require.True(t, acquired)
	require.Equal(t, int64(1), gen)

	// Simulate another message arriving while we hold the lock.
	// This bumps the generation but doesn't acquire (active lock held).
	acquired2, gen2 := locker.Acquire(ctx, projectID, listID)
	assert.False(t, acquired2)
	assert.Equal(t, int64(2), gen2)

	// Release first (matches handler flow: release before checking generation).
	locker.Release(ctx, projectID, listID)

	// Generation should detect the mismatch.
	latest := locker.Generation(ctx, projectID, listID)
	assert.Equal(t, int64(2), latest, "should return latest generation for re-loop")
	assert.NotEqual(t, gen, latest, "generation should differ from snapshot")

	// Re-acquire for the re-loop.
	acquired3, gen3 := locker.Acquire(ctx, projectID, listID)
	assert.True(t, acquired3, "should be able to re-acquire after release")
	assert.Equal(t, int64(3), gen3, "generation should increment on re-acquire")
}

func TestRecomputeLockerFullLoopCycle(t *testing.T) {
	t.Parallel()

	client := newTestRedisClient(t)
	prefix := testPrefix(t)
	locker := NewRecomputeLocker(client, prefix)
	ctx := context.Background()

	projectID := uuid.New()
	listID := uuid.New()

	// Step 1: First message acquires.
	acquired, gen := locker.Acquire(ctx, projectID, listID)
	require.True(t, acquired)
	require.Equal(t, int64(1), gen)

	// Step 2: Two more messages arrive while we are "recomputing".
	locker.Acquire(ctx, projectID, listID) //nolint:errcheck
	locker.Acquire(ctx, projectID, listID) //nolint:errcheck

	// Step 3: Release, then check — generation advanced from 1 to 3.
	locker.Release(ctx, projectID, listID)
	latest := locker.Generation(ctx, projectID, listID)
	assert.Equal(t, int64(3), latest, "should detect generation advanced to 3")
	assert.NotEqual(t, gen, latest)

	// Step 4: Re-acquire for re-loop.
	acquired2, gen2 := locker.Acquire(ctx, projectID, listID)
	require.True(t, acquired2)
	// gen2 is 4 because re-acquire INCRs the counter.

	// Step 5: No new messages during second recompute. Release and check.
	locker.Release(ctx, projectID, listID)
	latest2 := locker.Generation(ctx, projectID, listID)
	assert.Equal(t, gen2, latest2, "generation should be stable after re-loop")

	// Step 6: Done — lock is already released, new acquire should succeed.
	acquired3, gen3 := locker.Acquire(ctx, projectID, listID)
	assert.True(t, acquired3)
	assert.Equal(t, int64(5), gen3, "generation counter persists across cycles")
}

func TestRecomputeLockerRelease(t *testing.T) {
	t.Parallel()

	client := newTestRedisClient(t)
	prefix := testPrefix(t)
	locker := NewRecomputeLocker(client, prefix)
	ctx := context.Background()

	projectID := uuid.New()
	listID := uuid.New()

	acquired, _ := locker.Acquire(ctx, projectID, listID)
	require.True(t, acquired)

	// Explicit release (simulating error path).
	locker.Release(ctx, projectID, listID)

	// Lock should be available now.
	acquired2, _ := locker.Acquire(ctx, projectID, listID)
	assert.True(t, acquired2, "lock should be available after explicit release")
}

func TestRecomputeLockerReleaseIdempotent(t *testing.T) {
	t.Parallel()

	client := newTestRedisClient(t)
	prefix := testPrefix(t)
	locker := NewRecomputeLocker(client, prefix)
	ctx := context.Background()

	projectID := uuid.New()
	listID := uuid.New()

	// Releasing a lock that was never acquired should not panic or error.
	assert.NotPanics(t, func() {
		locker.Release(ctx, projectID, listID)
	})

	// Releasing twice should also be fine.
	acquired, _ := locker.Acquire(ctx, projectID, listID)
	require.True(t, acquired)
	locker.Release(ctx, projectID, listID)
	assert.NotPanics(t, func() {
		locker.Release(ctx, projectID, listID)
	})
}

func TestRecomputeLockerGenerationMonotonicallyIncreases(t *testing.T) {
	t.Parallel()

	client := newTestRedisClient(t)
	prefix := testPrefix(t)
	locker := NewRecomputeLocker(client, prefix)
	ctx := context.Background()

	projectID := uuid.New()
	listID := uuid.New()

	var generations []int64
	for i := 0; i < 10; i++ {
		_, gen := locker.Acquire(ctx, projectID, listID)
		generations = append(generations, gen)

		// Release after each acquire to allow the next one.
		if i < 9 {
			locker.Release(ctx, projectID, listID)
		}
	}

	for i := 1; i < len(generations); i++ {
		assert.Greater(t, generations[i], generations[i-1],
			"generation %d should be greater than %d", generations[i], generations[i-1])
	}
}

func TestRecomputeLockerConcurrentAcquire(t *testing.T) {
	t.Parallel()

	client := newTestRedisClient(t)
	prefix := testPrefix(t)
	locker := NewRecomputeLocker(client, prefix)
	ctx := context.Background()

	projectID := uuid.New()
	listID := uuid.New()

	const goroutines = 20
	var (
		mu           sync.Mutex
		acquireCount int
		wg           sync.WaitGroup
	)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			acquired, _ := locker.Acquire(ctx, projectID, listID)
			if acquired {
				mu.Lock()
				acquireCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, 1, acquireCount, "exactly one goroutine should acquire the lock")

	// Verify all goroutines bumped the generation.
	gen, err := client.Get(ctx, locker.genKey(projectID, listID)).Int64()
	require.NoError(t, err)
	assert.Equal(t, int64(goroutines), gen, "all goroutines should have incremented the generation")
}

func TestRecomputeLockerConcurrentFullCycle(t *testing.T) {
	t.Parallel()

	client := newTestRedisClient(t)
	prefix := testPrefix(t)
	locker := NewRecomputeLocker(client, prefix)
	ctx := context.Background()

	projectID := uuid.New()
	listID := uuid.New()

	// Simulate the full handler flow: one goroutine acquires and loops,
	// others bump the generation and exit.
	const producers = 10
	var (
		wg             sync.WaitGroup
		recomputeCount int64
		mu             sync.Mutex
		handlerReady   = make(chan struct{})
	)

	// Ensure the handler acquires the lock before any producer starts,
	// otherwise a zero-delay producer can win the SetNX race and the
	// handler never enters the recompute loop.
	acquired, gen := locker.Acquire(ctx, projectID, listID)
	require.True(t, acquired, "handler must win initial acquire")

	wg.Add(producers + 1)

	// The handler goroutine.
	go func() {
		defer wg.Done()

		gen := gen
		close(handlerReady)

		for {
			mu.Lock()
			recomputeCount++
			mu.Unlock()

			// Brief pause to let producer goroutines bump the generation.
			time.Sleep(5 * time.Millisecond)

			locker.Release(ctx, projectID, listID)

			latest := locker.Generation(ctx, projectID, listID)
			if latest == gen {
				return
			}

			var reacquired bool
			reacquired, gen = locker.Acquire(ctx, projectID, listID)
			if !reacquired {
				return
			}
		}
	}()

	// Producer goroutines that bump the generation while the handler runs.
	for i := 0; i < producers; i++ {
		go func(delay int) {
			defer wg.Done()
			<-handlerReady
			time.Sleep(time.Duration(delay) * time.Millisecond)
			locker.Acquire(ctx, projectID, listID) //nolint:errcheck
		}(i)
	}

	wg.Wait()

	mu.Lock()
	count := recomputeCount
	mu.Unlock()

	// The handler should have recomputed at least once and at most a few times —
	// significantly fewer than the number of producers.
	assert.GreaterOrEqual(t, count, int64(1), "should recompute at least once")
	assert.LessOrEqual(t, count, int64(producers), "should recompute fewer times than messages")
}

func TestRecomputeLockerActiveTTL(t *testing.T) {
	t.Parallel()

	client := newTestRedisClient(t)
	prefix := testPrefix(t)
	locker := NewRecomputeLocker(client, prefix)
	ctx := context.Background()

	projectID := uuid.New()
	listID := uuid.New()

	acquired, _ := locker.Acquire(ctx, projectID, listID)
	require.True(t, acquired)

	// Verify the active key has a TTL set.
	ttl, err := client.TTL(ctx, locker.activeKey(projectID, listID)).Result()
	require.NoError(t, err)
	assert.Greater(t, ttl, time.Duration(0), "active key should have a positive TTL")
	assert.LessOrEqual(t, ttl, RecomputeActiveTTL, "TTL should not exceed the configured maximum")
}

func TestRecomputeLockerKeyFormat(t *testing.T) {
	t.Parallel()

	projectID := uuid.New()
	listID := uuid.New()

	locker := NewRecomputeLocker(nil, "myprefix:")

	activeKey := locker.activeKey(projectID, listID)
	genKey := locker.genKey(projectID, listID)

	expectedActive := fmt.Sprintf("myprefix:recompute:active:%s:%s", projectID, listID)
	expectedGen := fmt.Sprintf("myprefix:recompute:gen:%s:%s", projectID, listID)

	assert.Equal(t, expectedActive, activeKey)
	assert.Equal(t, expectedGen, genKey)
}

func TestRecomputeLockerBurstDeduplication(t *testing.T) {
	t.Parallel()

	client := newTestRedisClient(t)
	prefix := testPrefix(t)
	locker := NewRecomputeLocker(client, prefix)
	ctx := context.Background()

	projectID := uuid.New()
	listID := uuid.New()

	// Acquire and simulate a burst of 50 messages.
	acquired, gen := locker.Acquire(ctx, projectID, listID)
	require.True(t, acquired)
	require.Equal(t, int64(1), gen)

	for i := 0; i < 49; i++ {
		acq, _ := locker.Acquire(ctx, projectID, listID)
		assert.False(t, acq)
	}

	// First check: release, then read generation — advanced from 1 to 50.
	locker.Release(ctx, projectID, listID)
	latest := locker.Generation(ctx, projectID, listID)
	assert.Equal(t, int64(50), latest)
	assert.NotEqual(t, gen, latest)

	// Re-acquire for re-loop.
	acquired2, gen2 := locker.Acquire(ctx, projectID, listID)
	require.True(t, acquired2)

	// No more messages arrive during second recompute — release and check.
	locker.Release(ctx, projectID, listID)
	latest2 := locker.Generation(ctx, projectID, listID)
	assert.Equal(t, gen2, latest2, "generation should be stable")

	// Done — lock already released, new acquire should succeed.
	acquired3, gen3 := locker.Acquire(ctx, projectID, listID)
	assert.True(t, acquired3)
	assert.Equal(t, int64(52), gen3)
}

func TestRecomputeLockerActiveKeyLifecycle(t *testing.T) {
	t.Parallel()

	client := newTestRedisClient(t)
	prefix := testPrefix(t)
	locker := NewRecomputeLocker(client, prefix)
	ctx := context.Background()

	projectID := uuid.New()
	listID := uuid.New()

	// Before acquire: no active key.
	exists, err := client.Exists(ctx, locker.activeKey(projectID, listID)).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists)

	// After acquire: active key exists.
	acquired, _ := locker.Acquire(ctx, projectID, listID)
	require.True(t, acquired)

	exists, err = client.Exists(ctx, locker.activeKey(projectID, listID)).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), exists, "active key must exist after acquire")

	// After release: active key gone.
	locker.Release(ctx, projectID, listID)

	exists, err = client.Exists(ctx, locker.activeKey(projectID, listID)).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), exists, "active key must be deleted after release")
}

func TestRecomputeLockerGenerationSurvivesRelease(t *testing.T) {
	t.Parallel()

	client := newTestRedisClient(t)
	prefix := testPrefix(t)
	locker := NewRecomputeLocker(client, prefix)
	ctx := context.Background()

	projectID := uuid.New()
	listID := uuid.New()

	// Acquire and release several times — the generation counter should persist.
	for i := 0; i < 5; i++ {
		acquired, gen := locker.Acquire(ctx, projectID, listID)
		require.True(t, acquired)
		assert.Equal(t, int64(i+1), gen)
		locker.Release(ctx, projectID, listID)
	}

	// Generation counter should still be readable.
	gen := locker.Generation(ctx, projectID, listID)
	assert.Equal(t, int64(5), gen)
}

func TestRecomputeLockerGenerationBeforeAnyAcquire(t *testing.T) {
	t.Parallel()

	client := newTestRedisClient(t)
	prefix := testPrefix(t)
	locker := NewRecomputeLocker(client, prefix)
	ctx := context.Background()

	projectID := uuid.New()
	listID := uuid.New()

	// Reading generation before any acquire should return 0 (key doesn't exist).
	gen := locker.Generation(ctx, projectID, listID)
	assert.Equal(t, int64(0), gen)
}
