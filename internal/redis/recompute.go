package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RecomputeActiveTTL is the maximum time a recompute lock can be held before
// it expires automatically. This acts as a safety net in case the holder
// crashes or is killed without releasing the lock.
const RecomputeActiveTTL = 60 * time.Second

// RecomputeLocker manages distributed deduplication for list recomputes using
// a generation counter and an active lock in Redis.
//
// The mechanism works as follows:
//
//   - Every incoming recompute message increments a per-list generation counter
//     (INCR). This is a cheap, atomic operation.
//   - The first handler to also acquire the active lock (SET NX EX) proceeds
//     with the actual recomputation.
//   - Handlers that fail to acquire the active lock return immediately — their
//     INCR already signalled that new data may exist.
//   - After each recompute the handler releases the active lock, then reads
//     the generation counter and compares it to its own snapshot. If the
//     counter has advanced, the handler re-acquires the lock and loops. If
//     not, the handler is done.
//   - Releasing before checking eliminates the race window between reading
//     the counter and releasing the lock. Any message that arrives after
//     the release will successfully acquire the lock itself rather than
//     being silently skipped.
//
// A nil *RecomputeLocker (or one with a nil Redis client) is safe to use and
// fails open: Acquire always succeeds, and the handler behaves as if no
// deduplication is configured.
type RecomputeLocker struct {
	redis  *redis.Client
	prefix string
}

// NewRecomputeLocker creates a new locker backed by the given Redis client.
// If client is nil the locker is a no-op (fail-open).
func NewRecomputeLocker(client *redis.Client, prefix string) *RecomputeLocker {
	return &RecomputeLocker{redis: client, prefix: prefix}
}

func (l *RecomputeLocker) activeKey(projectID, listID uuid.UUID) string {
	return fmt.Sprintf("%srecompute:active:%s:%s", l.prefix, projectID, listID)
}

func (l *RecomputeLocker) genKey(projectID, listID uuid.UUID) string {
	return fmt.Sprintf("%srecompute:gen:%s:%s", l.prefix, projectID, listID)
}

// Acquire increments the generation counter for the given list and attempts to
// acquire the active lock.
//
// It returns:
//   - acquired: true if this caller should perform the recompute.
//   - generation: the generation number captured by this call.
//
// When acquired is false the caller should ACK the message and return — the
// current lock holder will observe the bumped generation and re-loop.
func (l *RecomputeLocker) Acquire(ctx context.Context, projectID, listID uuid.UUID) (acquired bool, generation int64) {
	if l == nil || l.redis == nil {
		return true, 0
	}

	gen, err := l.redis.Incr(ctx, l.genKey(projectID, listID)).Result()
	if err != nil {
		return true, 0
	}

	ok, err := l.redis.SetNX(ctx, l.activeKey(projectID, listID), "1", RecomputeActiveTTL).Result()
	if err != nil {
		return true, gen
	}

	return ok, gen
}

// Generation reads the current generation counter for the given list and
// releases the active lock. Returns generation: the latest generation number.
// The caller should compare this to its own snapshot to decide whether another
// recompute loop is needed.
func (l *RecomputeLocker) Generation(ctx context.Context, projectID, listID uuid.UUID) (generation int64) {
	if l == nil || l.redis == nil {
		return 0
	}

	gen, err := l.redis.Get(ctx, l.genKey(projectID, listID)).Int64()
	if err != nil {
		return 0
	}

	return gen
}

// Release unconditionally removes the active lock. This should be called
// after a recompute completes and the generation has been read, or when the
// recompute fails and the message will be NAK'd, allowing a retry to
// re-acquire the lock immediately.
func (l *RecomputeLocker) Release(ctx context.Context, projectID, listID uuid.UUID) {
	if l == nil || l.redis == nil {
		return
	}

	l.redis.Del(ctx, l.activeKey(projectID, listID))
}
