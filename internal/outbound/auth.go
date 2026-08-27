package outbound

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"

	"gopkg.in/yaml.v3"
)

// Strategy authenticates a single outbound request. Implementations are
// constructed once at config-load time and must be safe for concurrent use;
// [OAuth2ClientCredentials] in particular caches a token across calls.
type Strategy interface {
	// Apply mutates req to carry the strategy's credential. It may perform I/O
	// (an OAuth2 token fetch), so it takes a context and can fail.
	Apply(ctx context.Context, req *http.Request) error
}

// AuthConfig is the configured authentication for one destination. Type selects
// the strategy and Config carries its type-specific settings, decoded lazily so
// the strategy set stays open for extension without this struct growing a field
// per strategy.
type AuthConfig struct {
	Type   string    `yaml:"type"`
	Config yaml.Node `yaml:"config"`
}

// Builder constructs a strategy from its type-specific configuration. deps
// carries what a strategy may need beyond its own settings — the OAuth2
// strategy needs a guarded HTTP client for the token endpoint.
type Builder func(node yaml.Node, deps StrategyDeps) (Strategy, error)

// StrategyDeps is the environment handed to a [Builder].
type StrategyDeps struct {
	// HTTPClient is a client already constrained by the destination's network
	// policy. A strategy that calls out (OAuth2) must use it rather than
	// http.DefaultClient, so the token endpoint is guarded exactly as the
	// destination is.
	HTTPClient *http.Client
}

var (
	buildersMu sync.RWMutex
	builders   = map[string]Builder{}
)

// RegisterStrategy adds an authentication strategy under name. It panics on a
// duplicate registration, which can only be a programming error at init time.
func RegisterStrategy(name string, build Builder) {
	buildersMu.Lock()
	defer buildersMu.Unlock()
	if _, exists := builders[name]; exists {
		panic("outbound: auth strategy already registered: " + name)
	}
	builders[name] = build
}

// StrategyNames returns the registered strategy names in sorted order, for
// error messages that tell an operator what they could have written instead.
func StrategyNames() []string {
	buildersMu.RLock()
	defer buildersMu.RUnlock()
	names := make([]string, 0, len(builders))
	for name := range builders {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// BuildStrategy resolves cfg into a strategy. An empty or "none" type yields a
// nil strategy, which [Client] treats as unauthenticated.
func BuildStrategy(cfg AuthConfig, deps StrategyDeps) (Strategy, error) {
	if cfg.Type == "" || cfg.Type == "none" {
		return nil, nil
	}

	buildersMu.RLock()
	build, ok := builders[cfg.Type]
	buildersMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown auth type %q (known: %v)", cfg.Type, StrategyNames())
	}

	strategy, err := build(cfg.Config, deps)
	if err != nil {
		return nil, fmt.Errorf("auth %q: %w", cfg.Type, err)
	}
	return strategy, nil
}

// decodeStrategyConfig decodes a strategy's config node into out, rejecting
// unknown keys so a typo in a credential field fails at boot rather than
// silently producing a request with no credential on it.
func decodeStrategyConfig(node yaml.Node, out any) error {
	if node.IsZero() {
		return fmt.Errorf("missing config block")
	}
	raw, err := yaml.Marshal(&node)
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	return nil
}
