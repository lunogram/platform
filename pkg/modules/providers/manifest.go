package providers

import (
	"time"

	"github.com/lunogram/platform/pkg/modules"
)

var ProjectMaxRateLimit = Limit{
	Requests: 250,
	Window:   time.Minute,
}

// ProviderManifest is the manifest for provider modules.
type ProviderManifest struct {
	Metadata modules.Metadata `json:"metadata"`
	Website  string           `json:"website,omitempty"`
	Version  string           `json:"version"`
	License  string           `json:"license"`
	Author   modules.Author   `json:"author"`
	Spec     ProviderSpec     `json:"spec"`
}

// GetMetadata implements modules.Manifest.
func (m ProviderManifest) GetMetadata() modules.Metadata {
	return m.Metadata
}

// RateLimit defines a rate limit as a number of allowed requests within a time
// interval.
//
// Override indicates whether the limit may be changed by users at the
// project/provider level. When Override is false the manifest value is
// authoritative and any DB-level override is ignored (some upstream services
// simply do not allow the rate to be adjusted).
type RateLimit struct {
	Limit    int    `json:"limit"`              // maximum number of requests per interval
	Interval string `json:"interval,omitempty"` // time.Duration string (e.g. "1s", "1m", "1h"); defaults to "1s"
	Override bool   `json:"override,omitempty"` // true if users may override this limit per-provider
}

// ParseInterval parses the Interval field as a time.Duration.
// Returns time.Second when Interval is empty or malformed.
func (r RateLimit) ParseInterval() time.Duration {
	if r.Interval == "" {
		return time.Second
	}
	d, err := time.ParseDuration(r.Interval)
	if err != nil || d <= 0 {
		return time.Second
	}
	return d
}

// ProviderSpec defines the specification for a provider module.
type ProviderSpec struct {
	Channels  []Channel           `json:"channels"`
	Config    *modules.JSONSchema `json:"config,omitempty"`
	Locked    bool                `json:"locked,omitempty"`
	Webhook   bool                `json:"webhook,omitempty"` // true if the module exports a webhook() function
	RateLimit *RateLimit          `json:"rate_limit,omitempty"`
}
