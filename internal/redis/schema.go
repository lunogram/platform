package redis

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// SchemaCacheTTL is the duration a schema fingerprint is remembered in Redis.
// After this period the fingerprint expires and the next event with the same
// data shape will trigger a fresh schema analysis. This acts as a safety net
// for schema table repairs or bugs — even unchanged shapes get re-analysed at
// most once per TTL window.
const SchemaCacheTTL = 24 * time.Hour

// SchemaCache uses Redis to track which data shapes have already been analysed.
// Before publishing an event to a schema subject the caller computes a
// structural fingerprint of the data payload and checks whether it was already
// seen. If so, the publish is skipped entirely, avoiding all downstream work
// (NATS delivery, JSON deserialization, ParsePaths, and individual SQL
// INSERT … ON CONFLICT DO NOTHING statements).
//
// A nil *SchemaCache (or one with a nil Redis client) is safe to use and
// fails open: Seen always returns false so every event is analysed, matching
// the pre-optimisation behaviour.
type SchemaCache struct {
	redis  *redis.Client
	prefix string
}

// NewSchemaCache creates a new cache backed by the given Redis client.
// If client is nil the cache is a no-op (fail-open).
func NewSchemaCache(client *redis.Client, prefix string) *SchemaCache {
	return &SchemaCache{redis: client, prefix: prefix}
}

// Seen reports whether the data shape identified by entityKey + data has
// already been recorded. If not, it stores the fingerprint with a TTL and
// returns false so the caller proceeds with schema analysis. On Redis errors
// it returns false (fail-open) so the caller always falls through to analysis.
func (c *SchemaCache) Seen(ctx context.Context, namespace Namespace, projectID uuid.UUID, data map[string]any) bool {
	if c == nil || c.redis == nil {
		return false
	}

	fp := Fingerprint(data)
	key := fmt.Sprintf("%sschema:%s:%s:%s", c.prefix, namespace, projectID, fp)

	ok, err := c.redis.SetNX(ctx, key, "1", SchemaCacheTTL).Result()
	if err != nil {
		return false
	}

	// SetNX returns true when the key was newly created (first time seen).
	// We return the inverse: seen = !ok.
	return !ok
}

// Fingerprint computes a deterministic hash of the structural shape of a JSON
// data map. It walks the map recursively, collecting "path:type" strings in
// sorted order, and hashes the result with FNV-64a. Two payloads that share
// the same set of keys and value types produce the same fingerprint regardless
// of key order or concrete values.
func Fingerprint(data map[string]any) string {
	if len(data) == 0 {
		return "empty"
	}

	var entries []string
	collectFingerprint(&entries, "", data)
	sort.Strings(entries)

	h := fnv.New64a()
	for _, e := range entries {
		h.Write([]byte(e))
	}

	return fmt.Sprintf("%016x", h.Sum64())
}

func collectFingerprint(entries *[]string, prefix string, m map[string]any) {
	for key, val := range m {
		path := prefix + "." + key
		collectFingerprintValue(entries, path, val)
	}
}

func collectFingerprintValue(entries *[]string, path string, v any) {
	switch vv := v.(type) {
	case map[string]any:
		*entries = append(*entries, path+":object")
		collectFingerprint(entries, path, vv)
	case []any:
		*entries = append(*entries, path+":array")
		if len(vv) > 0 {
			childPath := path + "[]"
			collectFingerprintValue(entries, childPath, vv[0])
		}
	case string:
		*entries = append(*entries, path+":string")
	case float64:
		*entries = append(*entries, path+":number")
	case bool:
		*entries = append(*entries, path+":boolean")
	case nil:
		*entries = append(*entries, path+":null")
	}
}
