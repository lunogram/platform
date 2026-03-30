package providers

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ProviderKey returns the rate-limit key for a given provider.
func ProviderKey(providerID uuid.UUID) string {
	return "rl:provider:" + providerID.String()
}

// NewLimit creates a clamped Limit. If the computed requests-per-minute
// exceeds max, max is returned instead.
func NewLimit(requests int, window time.Duration, max Limit) Limit {
	limit := Limit{
		Requests: requests,
		Window:   window,
	}

	if !limit.Active() {
		return limit
	}

	rpm := float64(limit.Requests) / window.Minutes()
	mrpm := float64(max.Requests) / max.Window.Minutes()
	if rpm > mrpm {
		return max
	}

	return limit
}

// NewLimitWithKey creates a clamped Limit and attaches the given key.
func NewLimitWithKey(key string, requests int, window time.Duration, max Limit) Limit {
	l := NewLimit(requests, window, max)
	l.Key = key
	return l
}

// Limit represents a resolved, clamped rate limit ready to be checked against
// the limiter. A zero-value Limit (Requests == 0) means no rate limiting.
//
// JSON serialization uses milliseconds for the window to avoid the fragile
// nanosecond representation that time.Duration produces by default.
type Limit struct {
	Key      string        `json:"key,omitempty"`
	Requests int           `json:"requests,omitempty"`
	Window   time.Duration `json:"-"`
}

// Active reports whether this limit is configured (i.e. should be enforced).
func (l Limit) Active() bool {
	return l.Requests > 0 && l.Window > 0
}

// limitJSON is the wire representation used for JSON marshaling.
type limitJSON struct {
	Key      string `json:"key,omitempty"`
	Requests int    `json:"requests,omitempty"`
	WindowMs int64  `json:"window_ms,omitempty"`
}

// MarshalJSON implements json.Marshaler.
func (l Limit) MarshalJSON() ([]byte, error) {
	return json.Marshal(limitJSON{
		Key:      l.Key,
		Requests: l.Requests,
		WindowMs: l.Window.Milliseconds(),
	})
}

// UnmarshalJSON implements json.Unmarshaler.
func (l *Limit) UnmarshalJSON(data []byte) error {
	var v limitJSON
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	l.Key = v.Key
	l.Requests = v.Requests
	l.Window = time.Duration(v.WindowMs) * time.Millisecond
	return nil
}

// parseDuration parses a Go duration string, returning time.Second when the
// input is empty or malformed.
func parseDuration(s string) time.Duration {
	if s == "" {
		return time.Second
	}

	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return time.Second
	}

	return d
}

// ResolveLimit builds a rate-limit from the provider manifest, optionally
// overridden by per-provider DB values when the manifest allows it.
// The returned Limit carries key so consumers can use it directly.
func ResolveLimit(key string, manifest *RateLimit, requests int, window string) (result Limit) {
	if manifest != nil && manifest.Limit > 0 {
		result = NewLimitWithKey(key, manifest.Limit, manifest.ParseInterval(), ProjectMaxRateLimit)
	}

	if manifest != nil && manifest.Override && requests > 0 {
		result = NewLimitWithKey(key, requests, parseDuration(window), ProjectMaxRateLimit)
	}

	return result
}
