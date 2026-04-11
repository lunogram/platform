package redis

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFingerprint(t *testing.T) {
	t.Parallel()

	t.Run("empty map returns empty sentinel", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "empty", Fingerprint(map[string]any{}))
	})

	t.Run("nil map returns empty sentinel", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "empty", Fingerprint(nil))
	})

	t.Run("deterministic across calls", func(t *testing.T) {
		t.Parallel()
		data := map[string]any{
			"name": "Alice",
			"age":  float64(30),
		}
		fp1 := Fingerprint(data)
		fp2 := Fingerprint(data)
		assert.Equal(t, fp1, fp2)
	})

	t.Run("same structure different values produces same fingerprint", func(t *testing.T) {
		t.Parallel()
		a := map[string]any{"name": "Alice", "age": float64(30)}
		b := map[string]any{"name": "Bob", "age": float64(25)}
		assert.Equal(t, Fingerprint(a), Fingerprint(b))
	})

	t.Run("different structure produces different fingerprint", func(t *testing.T) {
		t.Parallel()
		a := map[string]any{"name": "Alice"}
		b := map[string]any{"name": "Alice", "age": float64(30)}
		assert.NotEqual(t, Fingerprint(a), Fingerprint(b))
	})

	t.Run("different types produce different fingerprint", func(t *testing.T) {
		t.Parallel()
		a := map[string]any{"value": "hello"}
		b := map[string]any{"value": float64(42)}
		assert.NotEqual(t, Fingerprint(a), Fingerprint(b))
	})

	t.Run("nested objects", func(t *testing.T) {
		t.Parallel()
		a := map[string]any{
			"user": map[string]any{
				"name": "Alice",
				"age":  float64(30),
			},
		}
		b := map[string]any{
			"user": map[string]any{
				"name": "Bob",
				"age":  float64(25),
			},
		}
		assert.Equal(t, Fingerprint(a), Fingerprint(b))
	})

	t.Run("nested objects different structure", func(t *testing.T) {
		t.Parallel()
		a := map[string]any{
			"user": map[string]any{
				"name": "Alice",
			},
		}
		b := map[string]any{
			"user": map[string]any{
				"name":  "Alice",
				"email": "alice@example.com",
			},
		}
		assert.NotEqual(t, Fingerprint(a), Fingerprint(b))
	})

	t.Run("arrays", func(t *testing.T) {
		t.Parallel()
		a := map[string]any{
			"tags": []any{"foo", "bar"},
		}
		b := map[string]any{
			"tags": []any{"baz"},
		}
		assert.Equal(t, Fingerprint(a), Fingerprint(b))
	})

	t.Run("array vs non-array differs", func(t *testing.T) {
		t.Parallel()
		a := map[string]any{"tags": []any{"foo"}}
		b := map[string]any{"tags": "foo"}
		assert.NotEqual(t, Fingerprint(a), Fingerprint(b))
	})

	t.Run("booleans", func(t *testing.T) {
		t.Parallel()
		a := map[string]any{"active": true}
		b := map[string]any{"active": false}
		assert.Equal(t, Fingerprint(a), Fingerprint(b))
	})

	t.Run("null values", func(t *testing.T) {
		t.Parallel()
		a := map[string]any{"value": nil}
		fp := Fingerprint(a)
		assert.NotEqual(t, "empty", fp)
	})

	t.Run("key order does not matter", func(t *testing.T) {
		t.Parallel()
		// Go maps have random iteration order, but let's also construct
		// them explicitly to be sure.
		a := map[string]any{"z": "val", "a": "val", "m": "val"}
		b := map[string]any{"a": "val", "m": "val", "z": "val"}
		assert.Equal(t, Fingerprint(a), Fingerprint(b))
	})
}

func TestSchemaCacheNilSafety(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	data := map[string]any{"key": "value"}

	t.Run("nil cache returns false", func(t *testing.T) {
		t.Parallel()
		var cache *SchemaCache
		assert.False(t, cache.Seen(ctx, User, uuid.New(), data))
	})

	t.Run("nil redis client returns false", func(t *testing.T) {
		t.Parallel()
		cache := NewSchemaCache(nil, "")
		assert.False(t, cache.Seen(ctx, User, uuid.New(), data))
	})
}

func TestSchemaCacheSeen(t *testing.T) {
	t.Parallel()

	client := newTestRedisClient(t)
	prefix := testPrefix(t)
	cache := NewSchemaCache(client, prefix)
	ctx := context.Background()

	t.Run("first call returns false, second returns true", func(t *testing.T) {
		t.Parallel()
		prefix := testPrefix(t)
		cache := NewSchemaCache(client, prefix)
		data := map[string]any{"name": "Alice", "age": float64(30)}

		projID := uuid.New()
		seen1 := cache.Seen(ctx, Event, projID, data)
		assert.False(t, seen1, "first encounter should not be seen")

		seen2 := cache.Seen(ctx, Event, projID, data)
		assert.True(t, seen2, "second encounter should be seen")
	})

	t.Run("different entity keys are independent", func(t *testing.T) {
		t.Parallel()
		prefix := testPrefix(t)
		cache := NewSchemaCache(client, prefix)
		data := map[string]any{"name": "Alice"}

		seen1 := cache.Seen(ctx, Event, uuid.New(), data)
		assert.False(t, seen1)

		seen2 := cache.Seen(ctx, Event, uuid.New(), data)
		assert.False(t, seen2, "different entity key should not be seen")
	})

	t.Run("same entity key different structure is not seen", func(t *testing.T) {
		t.Parallel()
		prefix := testPrefix(t)
		cache := NewSchemaCache(client, prefix)
		dataA := map[string]any{"name": "Alice"}
		dataB := map[string]any{"name": "Alice", "email": "alice@example.com"}

		projID := uuid.New()
		seen1 := cache.Seen(ctx, User, projID, dataA)
		assert.False(t, seen1)

		seen2 := cache.Seen(ctx, User, projID, dataB)
		assert.False(t, seen2, "different data structure should not be seen")
	})

	t.Run("same structure different values is seen", func(t *testing.T) {
		t.Parallel()
		prefix := testPrefix(t)
		cache := NewSchemaCache(client, prefix)
		dataA := map[string]any{"name": "Alice", "age": float64(30)}
		dataB := map[string]any{"name": "Bob", "age": float64(25)}

		projID := uuid.New()
		seen1 := cache.Seen(ctx, User, projID, dataA)
		assert.False(t, seen1)

		seen2 := cache.Seen(ctx, User, projID, dataB)
		assert.True(t, seen2, "same structure with different values should be seen")
	})

	t.Run("prefix isolation", func(t *testing.T) {
		t.Parallel()
		cacheA := NewSchemaCache(client, "prefixA:")
		cacheB := NewSchemaCache(client, "prefixB:")
		data := map[string]any{"key": "val"}

		projID := uuid.New()
		seenA := cacheA.Seen(ctx, User, projID, data)
		assert.False(t, seenA)

		seenB := cacheB.Seen(ctx, User, projID, data)
		assert.False(t, seenB, "different prefix should be independent")
	})

	_ = cache // suppress unused warning from the outer scope
}

func TestSchemaCacheKeyHasTTL(t *testing.T) {
	t.Parallel()

	client := newTestRedisClient(t)
	prefix := testPrefix(t)
	cache := NewSchemaCache(client, prefix)
	ctx := context.Background()

	data := map[string]any{"key": "value"}

	projID := uuid.New()
	seen := cache.Seen(ctx, Event, projID, data)
	require.False(t, seen)

	// Reconstruct the key to check its TTL.
	fp := Fingerprint(data)
	redisKey := fmt.Sprintf("%sschema:%s:%s:%s", prefix, Event, projID, fp)

	ttl, err := client.TTL(ctx, redisKey).Result()
	require.NoError(t, err)
	assert.Greater(t, ttl.Seconds(), float64(0), "key should have a positive TTL")
	assert.LessOrEqual(t, ttl, SchemaCacheTTL, "TTL should not exceed the configured maximum")
}
