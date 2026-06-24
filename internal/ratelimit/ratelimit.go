package ratelimit

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// slidingWindowScript is an atomic Lua script implementing a sliding-window
// rate limiter using a Redis sorted set.
//
// KEYS[1] = the rate-limit key
// ARGV[1] = current time in microseconds
// ARGV[2] = window size in microseconds
// ARGV[3] = max requests per window
//
// Returns {1, 0} if allowed, {0, retryAfterMicroseconds} if denied.
var slidingWindowScript = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local min_score = now - window

-- Remove expired entries
redis.call('ZREMRANGEBYSCORE', key, '-inf', min_score)

-- Count current entries
local count = redis.call('ZCARD', key)

if count < limit then
    -- Allowed: add this request
    redis.call('ZADD', key, now, now .. ':' .. math.random(1000000))
    redis.call('PEXPIRE', key, math.ceil(window / 1000))
    return {1, 0}
else
    -- Denied: find oldest entry to calculate retryAfter
    local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
    local retry_after = 0
    if #oldest >= 2 then
        retry_after = tonumber(oldest[2]) + window - now
        if retry_after < 0 then retry_after = 0 end
    end
    return {0, retry_after}
end
`)

// reserveScript is an atomic Lua script that always reserves a slot and
// returns the delay (in microseconds) the caller must wait before acting.
//
// Unlike slidingWindowScript which rejects excess requests, this script
// guarantees a reservation so the caller can schedule its work for a
// precise future instant without re-publishing.
//
// Algorithm:
//  1. Remove entries whose scheduled time is older than (now - window).
//  2. Count remaining entries.
//  3. If count < limit: add an entry at "now" and return delay = 0.
//  4. If count >= limit: find the newest entry (highest score), compute
//     the next slot as (newest + window/limit), add it, and return the
//     delay = next_slot - now.
//
// The sorted set contains both past (already executed) and future
// (reserved) timestamps. Past entries expire naturally via step 1.
//
// KEYS[1] = the rate-limit key
// ARGV[1] = current time in microseconds
// ARGV[2] = window size in microseconds
// ARGV[3] = max requests per window
var reserveScript = redis.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])
local min_score = now - window

-- Remove entries that have fully left the window.
redis.call('ZREMRANGEBYSCORE', key, '-inf', min_score)

local count = redis.call('ZCARD', key)

if count < limit then
    -- Slot available right now.
    redis.call('ZADD', key, now, now .. ':' .. math.random(1000000))
    redis.call('PEXPIRE', key, math.ceil(window / 1000) * 2)
    return 0
end

-- Window is full: find the latest reserved slot and schedule after it.
local newest = redis.call('ZRANGE', key, -1, -1, 'WITHSCORES')
local last_score = now
if #newest >= 2 then
    last_score = tonumber(newest[2])
end

-- Space between slots to stay within the rate limit.
local spacing = math.ceil(window / limit)
local next_slot = last_score + spacing

-- Ensure we never schedule in the past.
if next_slot < now then
    next_slot = now
end

redis.call('ZADD', key, next_slot, next_slot .. ':' .. math.random(1000000))
-- Set expiry to cover the furthest reservation plus one full window.
local ttl_ms = math.ceil((next_slot - now + window) / 1000)
redis.call('PEXPIRE', key, ttl_ms)

return next_slot - now
`)

// Limiter provides per-key rate limiting backed by Redis.
// A nil *Limiter is safe to use and allows all requests.
type Limiter struct {
	client *redis.Client
	prefix string
	logger *zap.Logger
}

// New creates a new rate limiter. If client is nil, Allow always permits.
func New(client *redis.Client, prefix string, logger *zap.Logger) *Limiter {
	return &Limiter{
		client: client,
		prefix: prefix,
		logger: logger,
	}
}

// Allow checks whether a request identified by key is permitted under the
// given rate limit (limit requests per window).
//
// Returns:
//   - allowed: true if the request may proceed
//   - retryAfter: when denied, the duration until a slot opens up
//   - err: only returned on unexpected Redis errors (callers should fail-open)
//
// A nil receiver or nil Redis client always returns allowed=true (fail-open).
func (l *Limiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, retryAfter time.Duration, err error) {
	if l == nil || l.client == nil {
		return true, 0, nil
	}

	nowMicro := time.Now().UnixMicro()
	windowMicro := window.Microseconds()

	result, err := slidingWindowScript.Run(ctx, l.client,
		[]string{l.prefix + key},
		nowMicro,
		windowMicro,
		limit,
	).Int64Slice()

	if err != nil {
		l.logger.Warn("rate limiter redis error, failing open", zap.Error(err), zap.String("key", key))
		return true, 0, nil
	}

	if len(result) < 2 {
		l.logger.Warn("rate limiter unexpected result length, failing open", zap.Int("len", len(result)), zap.String("key", key))
		return true, 0, nil
	}

	if result[0] == 1 {
		return true, 0, nil
	}

	retryAfterMicro := result[1]
	if retryAfterMicro <= 0 {
		// Edge case: window exactly full but oldest is about to expire.
		retryAfterMicro = 1000 // 1ms minimum
	}

	return false, time.Duration(retryAfterMicro) * time.Microsecond, nil
}

// Reserve atomically claims the next available rate-limit slot for key.
//
// If a slot is available right now, delay is 0 and the caller may proceed
// immediately. Otherwise the returned delay indicates exactly how long the
// caller must wait before its reserved slot opens. Unlike Allow, Reserve
// never rejects a request — it always guarantees a future slot.
//
// Callers are expected to schedule their work (e.g. via NATS message
// scheduling) for time.Now().Add(delay) when delay > 0.
//
// A nil receiver or nil Redis client always returns delay=0 (fail-open).
func (l *Limiter) Reserve(ctx context.Context, key string, limit int, window time.Duration) (delay time.Duration, err error) {
	if l == nil || l.client == nil {
		return 0, nil
	}

	nowMicro := time.Now().UnixMicro()
	windowMicro := window.Microseconds()

	result, err := reserveScript.Run(ctx, l.client,
		[]string{l.prefix + key},
		nowMicro,
		windowMicro,
		limit,
	).Int64()

	if err != nil {
		l.logger.Warn("rate limiter redis error, failing open", zap.Error(err), zap.String("key", key))
		return 0, nil
	}

	if result <= 0 {
		return 0, nil
	}

	return time.Duration(result) * time.Microsecond, nil
}
