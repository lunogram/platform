package webhook

import (
	"fmt"
	"time"

	"github.com/lunogram/platform/internal/outbound"
	"go.uber.org/zap"
)

// LegacyEnv holds the pre-config-file webhook settings.
//
// Deprecated: these environment variables predate file-based hook
// configuration. They can express exactly one subscriber per event and no
// authentication, templating, retry or network policy at all. They are honoured
// so existing deployments keep working, and synthesised into an equivalent
// in-memory configuration; set WEBHOOK_CONFIG_FILE instead.
type LegacyEnv struct {
	ProjectCreatedURL     string
	ProjectCreatedTimeout time.Duration
	EmailTemplatesURL     string
	EmailTemplatesTimeout time.Duration

	// ForwardAuthorization restores the previous implementation's forwarding of
	// the caller's Authorization header to the project.created receiver.
	//
	// Deprecated: the old code copied *every* inbound header, including the
	// caller's bearer token, onto a request to an operator-configured URL. That
	// is a credential leak, so the synthesised configuration does not forward
	// anything unless this is explicitly set — and then only the one header a
	// receiver could plausibly have depended on. Receivers should authenticate
	// the platform with their own credential and read the triggering identity
	// from ctx.actor instead.
	ForwardAuthorization bool
}

// configured reports whether any legacy variable is set.
func (l LegacyEnv) configured() bool {
	return l.ProjectCreatedURL != "" || l.EmailTemplatesURL != ""
}

// Config synthesises the equivalent hook configuration for the legacy
// variables, along with the deprecation warnings an operator should see.
func (l LegacyEnv) Config() (*Config, []string) {
	cfg := &Config{Version: ConfigVersion, Hooks: map[string][]HookConfig{}}
	warnings := []string{
		"WEBHOOK_PROJECT_CREATED_URL / WEBHOOK_EMAIL_TEMPLATES_URL are deprecated; " +
			"move them into a WEBHOOK_CONFIG_FILE (see docs/settings/webhooks)",
	}

	if l.ProjectCreatedURL != "" {
		hook := HookConfig{
			ID:      "legacy-project-created",
			URL:     l.ProjectCreatedURL,
			Method:  "POST",
			Timeout: l.ProjectCreatedTimeout,
			// The previous implementation returned the delivery error to the
			// API caller, failing project creation. Preserving that is the
			// point of the compatibility path.
			CanInterrupt: true,
			// Legacy URLs were never SSRF-checked, and deployments point them
			// at in-cluster receivers. Refusing to start on a URL that worked
			// yesterday would make this an upgrade break rather than a
			// compatibility path.
			Network: &outbound.Network{AllowPrivate: true, AllowHTTP: true},
		}
		if l.ForwardAuthorization {
			hook.ForwardHeaders = []string{"Authorization"}
			warnings = append(warnings,
				"WEBHOOK_LEGACY_FORWARD_AUTHORIZATION is set: the caller's Authorization header is "+
					"forwarded to WEBHOOK_PROJECT_CREATED_URL, granting that endpoint the caller's API privileges")
		} else {
			warnings = append(warnings,
				"WEBHOOK_PROJECT_CREATED_URL no longer receives the caller's request headers; "+
					"a receiver that relied on the forwarded Authorization header must be given its own "+
					"credential, or WEBHOOK_LEGACY_FORWARD_AUTHORIZATION=true set as a temporary bridge")
		}
		cfg.Hooks[ProjectCreated.Name] = []HookConfig{hook}
		cfg.Defaults.MaxDispatchTime = maxDuration(l.ProjectCreatedTimeout, DefaultMaxDispatchTime)
	}

	if l.EmailTemplatesURL != "" {
		cfg.EmailTemplates = &GalleryConfig{
			URL:     l.EmailTemplatesURL,
			Timeout: l.EmailTemplatesTimeout,
			Network: outbound.Network{AllowPrivate: true, AllowHTTP: true},
		}
		warnings = append(warnings,
			"WEBHOOK_EMAIL_TEMPLATES_URL no longer receives the caller's request headers; "+
				"configure email_templates.auth if the gallery requires authentication")
	}

	return cfg, warnings
}

// Load resolves the effective configuration from the three places it can come
// from, in order of precedence: the webhook section of the node configuration
// file, a standalone WEBHOOK_CONFIG_FILE, and the deprecated single-URL
// variables.
//
// A lower-precedence source that is also configured is ignored and said to be
// ignored, rather than merged into something none of them describes.
func Load(inline *Config, path string, legacy LegacyEnv) (*Config, []string, error) {
	if inline != nil {
		if inline.Version != ConfigVersion {
			return nil, nil, fmt.Errorf("webhook config version must be %q, got %q", ConfigVersion, inline.Version)
		}
		var warnings []string
		if path != "" {
			warnings = append(warnings,
				"the node configuration carries a webhook.outbound section, so WEBHOOK_CONFIG_FILE is ignored")
		}
		if legacy.configured() {
			warnings = append(warnings,
				"the node configuration carries a webhook.outbound section, so the deprecated "+
					"WEBHOOK_PROJECT_CREATED_URL / WEBHOOK_EMAIL_TEMPLATES_URL variables are ignored")
		}
		return inline, warnings, nil
	}

	if path != "" {
		cfg, err := LoadConfigFile(path)
		if err != nil {
			return nil, nil, err
		}
		var warnings []string
		if legacy.configured() {
			warnings = append(warnings,
				"WEBHOOK_CONFIG_FILE is set, so the deprecated WEBHOOK_PROJECT_CREATED_URL / "+
					"WEBHOOK_EMAIL_TEMPLATES_URL variables are ignored")
		}
		return cfg, warnings, nil
	}

	if !legacy.configured() {
		return nil, nil, nil
	}

	cfg, warnings := legacy.Config()
	return cfg, warnings, nil
}

// NewEngine loads the configuration and compiles it, logging any deprecation
// and policy warnings. It is the single entry point wiring uses.
func NewEngine(logger *zap.Logger, inline *Config, path string, legacy LegacyEnv) (*Engine, error) {
	cfg, warnings, err := Load(inline, path, legacy)
	if err != nil {
		return nil, err
	}
	for _, warning := range warnings {
		logger.Warn(warning)
	}

	engine, err := New(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("webhook: %w", err)
	}
	return engine, nil
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
