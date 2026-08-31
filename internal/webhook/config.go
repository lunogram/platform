package webhook

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/lunogram/platform/internal/configfile"
	"github.com/lunogram/platform/internal/outbound"
	"gopkg.in/yaml.v3"
)

// ConfigVersion is the only schema version this build understands. It is
// required in the file so a future incompatible schema can be introduced
// without having to guess what an existing file meant.
const ConfigVersion = "v1"

// DefaultMaxDispatchTime bounds every hook fired for one event, in aggregate.
// It exists because per-call timeouts do not compose: a 30s timeout with four
// attempts across three hooks is six minutes of an API caller waiting. The
// dispatch deadline is the number a human actually experiences.
const DefaultMaxDispatchTime = 30 * time.Second

// Config is the parsed hook configuration file.
type Config struct {
	Version  string                  `yaml:"version"`
	Defaults Defaults                `yaml:"defaults"`
	Hooks    map[string][]HookConfig `yaml:"hooks"`

	// EmailTemplates configures the template gallery client. It lives in this
	// file because it shares the transport, auth and network sub-schemas, but
	// it is not a hook and is not bound to an event — see internal/gallery.
	EmailTemplates *GalleryConfig `yaml:"email_templates"`

	// baseDir is the directory the file was loaded from, used to resolve
	// relative file:// template references.
	baseDir string
}

// Defaults are applied to every hook that does not override them.
type Defaults struct {
	Timeout         time.Duration    `yaml:"timeout"`
	MaxDispatchTime time.Duration    `yaml:"max_dispatch_time"`
	Retry           outbound.Retry   `yaml:"retry"`
	Network         outbound.Network `yaml:"network"`
	Response        ResponseConfig   `yaml:"response"`
}

// ResponseConfig decides what the engine does with a hook's response.
//
// The default (both false) reads the body, discards it, and lets the status
// decide whether the hook succeeded. Parse additionally hands the body back to
// the caller as raw JSON, which is what makes CanInterrupt useful: a hook can
// influence the operation that triggered it. Ignore is the fire-and-forget
// end of the range — the response is not inspected at all and any completed
// call counts as a success, non-2xx included.
type ResponseConfig struct {
	Ignore bool `yaml:"ignore"`
	Parse  bool `yaml:"parse"`
}

// HookConfig is one subscriber bound to one event.
type HookConfig struct {
	// ID names the hook in logs and errors. It is required, because "hook 2 of
	// project.created failed" is not something an operator can act on.
	ID     string `yaml:"id"`
	URL    string `yaml:"url"`
	Method string `yaml:"method"`

	// Body is a file:// reference or an inline JSONNet snippet. When empty the
	// event's embedded default template is used.
	Body string `yaml:"body"`

	// CanInterrupt makes this hook's failure fail the operation that triggered
	// it. When false the hook is best-effort: failures are logged and swallowed,
	// and the originating operation succeeds regardless.
	CanInterrupt bool `yaml:"can_interrupt"`

	Timeout          time.Duration       `yaml:"timeout"`
	Network          *outbound.Network   `yaml:"network"`
	Response         *ResponseConfig     `yaml:"response"`
	Retry            *outbound.Retry     `yaml:"retry"`
	Auth             outbound.AuthConfig `yaml:"auth"`
	MaxResponseBytes int64               `yaml:"max_response_bytes"`

	// ForwardHeaders copies named headers from the inbound request onto the
	// outbound hook request.
	//
	// Deprecated: this exists only to keep receivers working that still expect
	// the caller's Authorization header, which the previous implementation
	// forwarded wholesale. Forwarding a caller's bearer token to an
	// operator-configured URL hands that URL the caller's full API privileges.
	// Configure the hook's own `auth` instead and read the triggering identity
	// from `ctx.actor` in the body template. Every entry here is warned about
	// at load, and the option is scheduled for removal.
	ForwardHeaders []string `yaml:"forward_headers"`
}

// GalleryConfig configures the email template gallery client.
type GalleryConfig struct {
	URL              string              `yaml:"url"`
	Timeout          time.Duration       `yaml:"timeout"`
	Network          outbound.Network    `yaml:"network"`
	Retry            *outbound.Retry     `yaml:"retry"`
	Auth             outbound.AuthConfig `yaml:"auth"`
	MaxResponseBytes int64               `yaml:"max_response_bytes"`
}

// ParseConfig decodes hook configuration from raw YAML. baseDir resolves
// relative file:// template references.
func ParseConfig(raw []byte, baseDir string) (*Config, error) {
	expanded, err := configfile.Interpolate(raw)
	if err != nil {
		return nil, err
	}

	cfg := &Config{baseDir: baseDir}
	dec := yaml.NewDecoder(bytes.NewReader(expanded))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil {
		return nil, fmt.Errorf("invalid webhook config: %w", err)
	}

	if cfg.Version != ConfigVersion {
		return nil, fmt.Errorf("webhook config version must be %q, got %q", ConfigVersion, cfg.Version)
	}

	return cfg, nil
}

// SetBaseDir records the directory relative file:// template references resolve
// against. A configuration parsed from its own file learns this from the path
// it was read from; one declared inline in the node configuration has to be
// told, because it was never a file of its own.
func (c *Config) SetBaseDir(dir string) {
	if c != nil {
		c.baseDir = dir
	}
}

// LoadConfigFile reads and parses a hook configuration file.
func LoadConfigFile(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("webhook config: %w", err)
	}
	cfg, err := ParseConfig(raw, filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("webhook config %s: %w", path, err)
	}
	return cfg, nil
}

// resolvedTimeout returns the effective per-attempt timeout for a hook.
func (c *Config) resolvedTimeout(hook HookConfig) time.Duration {
	if hook.Timeout > 0 {
		return hook.Timeout
	}
	if c.Defaults.Timeout > 0 {
		return c.Defaults.Timeout
	}
	return 10 * time.Second
}

// maxDispatchTime returns the aggregate budget for one event's dispatch.
func (c *Config) maxDispatchTime() time.Duration {
	if c.Defaults.MaxDispatchTime > 0 {
		return c.Defaults.MaxDispatchTime
	}
	return DefaultMaxDispatchTime
}

// resolvedNetwork returns the effective network policy for a hook.
func (c *Config) resolvedNetwork(hook HookConfig) outbound.Network {
	if hook.Network != nil {
		return *hook.Network
	}
	return c.Defaults.Network
}

// resolvedResponse returns the effective response handling for a hook.
func (c *Config) resolvedResponse(hook HookConfig) ResponseConfig {
	if hook.Response != nil {
		return *hook.Response
	}
	return c.Defaults.Response
}

// resolvedRetry returns the effective retry policy for a hook, layered over the
// file defaults and then over the built-in defaults.
func (c *Config) resolvedRetry(hook HookConfig, timeout time.Duration) outbound.Retry {
	retry := outbound.Retry{}
	if hook.Retry != nil {
		retry = *hook.Retry
	}
	return retry.WithDefaults(c.Defaults.Retry, timeout).WithDefaults(outbound.DefaultRetry(), timeout)
}
