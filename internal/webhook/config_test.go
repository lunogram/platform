package webhook

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestParseConfigRequiresVersion(t *testing.T) {
	t.Parallel()

	_, err := ParseConfig([]byte("hooks: {}\n"), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version")

	_, err = ParseConfig([]byte("version: v2\n"), "")
	require.Error(t, err)
}

// TestParseConfigRejectsUnknownFields covers every nesting level of the schema.
//
// Strict decoding matters unevenly. A typo'd network or retry setting fails
// closed — the guard stays on, the retry budget stays at its default — but a
// typo'd can_interrupt fails *open*: a hook the operator wrote to gate an
// operation would silently stop gating it. Rejecting the key at load time is
// what makes that impossible rather than merely unlikely.
func TestParseConfigRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"top level":            "version: v1\nhoks: {}\n",
		"defaults":             "version: v1\ndefaults: {netwrk: {allow_private: true}}\n",
		"defaults retry":       "version: v1\ndefaults: {retry: {max_attemps: 4}}\n",
		"defaults response":    "version: v1\ndefaults: {response: {igore: true}}\n",
		"defaults network":     "version: v1\ndefaults: {network: {allow_privat: true}}\n",
		"hook":                 "version: v1\nhooks:\n  test.event:\n    - {id: a, url: 'https://e.com', cn_interrupt: true}\n",
		"hook network":         "version: v1\nhooks:\n  test.event:\n    - {id: a, url: 'https://e.com', network: {allow_privat: true}}\n",
		"hook retry":           "version: v1\nhooks:\n  test.event:\n    - {id: a, url: 'https://e.com', retry: {max_attemps: 4}}\n",
		"hook response":        "version: v1\nhooks:\n  test.event:\n    - {id: a, url: 'https://e.com', response: {pasre: true}}\n",
		"email templates":      "version: v1\nemail_templates: {url: 'https://g.com', timout: 5s}\n",
		"email templates auth": "version: v1\nemail_templates: {url: 'https://g.com', network: {allow_privat: true}}\n",
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseConfig([]byte(body), "")
			require.Error(t, err, "a typo'd key must not be silently ignored")
			assert.Contains(t, err.Error(), "field")
		})
	}
}

// TestCanInterruptTypoIsRejected is the case strict decoding exists for: unlike
// a mistyped guard, a mistyped can_interrupt would fail open.
func TestCanInterruptTypoIsRejected(t *testing.T) {
	t.Parallel()

	_, err := ParseConfig([]byte(`version: v1
hooks:
  test.event:
    - {id: a, url: 'https://e.com', body: 'function(ctx) {}', can_intrrupt: true}
`), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "can_intrrupt")
}

func TestInterpolation(t *testing.T) {
	t.Run("expands set variables", func(t *testing.T) {
		t.Setenv("TEST_HOOK_SECRET", "s3cr3t")
		cfg, err := ParseConfig([]byte("version: v1\nhooks:\n  test.event:\n    - {id: a, url: 'https://e.com', auth: {type: api_key, config: {name: X, value: '${TEST_HOOK_SECRET}'}}, body: 'function(ctx) {}'}\n"), "")
		require.NoError(t, err)
		require.Len(t, cfg.Hooks["test.event"], 1)

		engine, err := New(cfg, zaptest.NewLogger(t))
		require.NoError(t, err)
		assert.True(t, engine.Enabled("test.event"))
	})

	t.Run("an unset variable is a load error", func(t *testing.T) {
		_, err := ParseConfig([]byte("version: v1\nhooks:\n  test.event:\n    - {id: a, url: '${DEFINITELY_UNSET_HOOK_VAR}'}\n"), "")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "DEFINITELY_UNSET_HOOK_VAR")
	})
}

// TestNewRejectsInvalidConfigurations asserts on the distinguishing part of
// each error, not merely that one occurred. Several of these cases would
// otherwise pass for the wrong reason — test.event has no embedded default
// template, so any case omitting `body` fails on that before reaching the
// validation it is named for.
func TestNewRejectsInvalidConfigurations(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		config string
		want   string
	}{
		{
			name: "unknown event",
			config: `version: v1
hooks:
  nope.event:
    - {id: a, url: 'https://e.com', body: 'function(ctx) {}'}
`,
			want: `unknown event "nope.event"`,
		},
		{
			name: "missing id",
			config: `version: v1
hooks:
  test.event:
    - {url: 'https://e.com', body: 'function(ctx) {}'}
`,
			want: "id is required",
		},
		{
			name: "duplicate id",
			config: `version: v1
hooks:
  test.event:
    - {id: a, url: 'https://e.com', body: 'function(ctx) {}'}
    - {id: a, url: 'https://e.com', body: 'function(ctx) {}'}
`,
			want: `duplicate hook id "a"`,
		},
		{
			name: "missing url",
			config: `version: v1
hooks:
  test.event:
    - {id: a, body: 'function(ctx) {}'}
`,
			want: "url is required",
		},
		{
			name: "unsafe url",
			config: `version: v1
hooks:
  test.event:
    - {id: a, url: 'https://169.254.169.254/meta', body: 'function(ctx) {}'}
`,
			want: "not a public address",
		},
		{
			name: "plaintext url without allow_http",
			config: `version: v1
hooks:
  test.event:
    - {id: a, url: 'http://receiver.example.com/hook', body: 'function(ctx) {}'}
`,
			want: "must use https",
		},
		{
			name: "broken template",
			config: `version: v1
hooks:
  test.event:
    - {id: a, url: 'https://e.com', body: 'function(ctx) { oops'}
`,
			want: "Expected token",
		},
		{
			name: "missing template file",
			config: `version: v1
hooks:
  test.event:
    - {id: a, url: 'https://e.com', body: 'file:///definitely/not/here.jsonnet'}
`,
			want: "no such file or directory",
		},
		{
			name: "no default template for an event",
			config: `version: v1
hooks:
  test.event:
    - {id: a, url: 'https://e.com'}
`,
			want: `no default body template for event "test.event"`,
		},
		{
			name: "unknown auth type",
			config: `version: v1
hooks:
  test.event:
    - {id: a, url: 'https://e.com', body: 'function(ctx) {}', auth: {type: kerberos}}
`,
			want: `unknown auth type "kerberos"`,
		},
		{
			name: "incomplete auth config",
			config: `version: v1
hooks:
  test.event:
    - {id: a, url: 'https://e.com', body: 'function(ctx) {}', auth: {type: api_key, config: {name: X-Token}}}
`,
			want: "value is required",
		},
		{
			name: "unknown field inside auth config",
			config: `version: v1
hooks:
  test.event:
    - {id: a, url: 'https://e.com', body: 'function(ctx) {}', auth: {type: api_key, config: {nmae: X, value: v}}}
`,
			want: "nmae",
		},
		{
			name: "contradictory response handling",
			config: `version: v1
hooks:
  test.event:
    - {id: a, url: 'https://e.com', body: 'function(ctx) {}', response: {ignore: true, parse: true}}
`,
			want: "mutually exclusive",
		},
		{
			name: "timeout exceeds the dispatch budget",
			config: `version: v1
defaults: {max_dispatch_time: 5s}
hooks:
  test.event:
    - {id: a, url: 'https://e.com', body: 'function(ctx) {}', timeout: 10s}
`,
			want: "exceeds the dispatch budget",
		},
		{
			name: "zero retry attempts",
			config: `version: v1
hooks:
  test.event:
    - {id: a, url: 'https://e.com', body: 'function(ctx) {}', retry: {max_attempts: -1}}
`,
			want: "max_attempts must be at least 1",
		},
		{
			name: "initial interval exceeds max interval",
			config: `version: v1
hooks:
  test.event:
    - {id: a, url: 'https://e.com', body: 'function(ctx) {}', retry: {initial_interval: 1m, max_interval: 1s}}
`,
			want: "initial_interval must not exceed max_interval",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cfg, err := ParseConfig([]byte(tc.config), "")
			if err == nil {
				_, err = New(cfg, zaptest.NewLogger(t))
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestLoadConfigFileResolvesRelativeTemplates(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hook.jsonnet"), []byte(`function(ctx) { e: ctx.event }`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hooks.yaml"), []byte(`version: v1
hooks:
  test.event:
    - {id: a, url: 'https://receiver.example.com/hook', body: 'file://hook.jsonnet'}
`), 0o600))

	cfg, err := LoadConfigFile(filepath.Join(dir, "hooks.yaml"))
	require.NoError(t, err)

	engine, err := New(cfg, zaptest.NewLogger(t))
	require.NoError(t, err)
	assert.True(t, engine.Enabled("test.event"))
}

func TestLegacyEnvSynthesis(t *testing.T) {
	t.Parallel()

	t.Run("project created becomes an interrupting hook", func(t *testing.T) {
		legacy := LegacyEnv{ProjectCreatedURL: "http://courier.internal/hook", ProjectCreatedTimeout: 20 * time.Second}
		cfg, warnings := legacy.Config()

		hooks := cfg.Hooks[ProjectCreated.Name]
		require.Len(t, hooks, 1)
		assert.True(t, hooks[0].CanInterrupt)
		assert.Empty(t, hooks[0].ForwardHeaders, "forwarding is off unless explicitly restored")
		assert.NotEmpty(t, warnings)
	})

	t.Run("forwarding is opt-in and warned about", func(t *testing.T) {
		legacy := LegacyEnv{ProjectCreatedURL: "http://courier.internal/hook", ForwardAuthorization: true}
		cfg, warnings := legacy.Config()

		assert.Equal(t, []string{"Authorization"}, cfg.Hooks[ProjectCreated.Name][0].ForwardHeaders)
		assert.Contains(t, joined(warnings), "Authorization header is forwarded")
	})

	t.Run("email templates becomes gallery config", func(t *testing.T) {
		legacy := LegacyEnv{EmailTemplatesURL: "https://gallery.example.com/templates", EmailTemplatesTimeout: 5 * time.Second}
		cfg, _ := legacy.Config()

		require.NotNil(t, cfg.EmailTemplates)
		assert.Equal(t, "https://gallery.example.com/templates", cfg.EmailTemplates.URL)
		assert.Equal(t, 5*time.Second, cfg.EmailTemplates.Timeout)
		assert.Empty(t, cfg.Hooks)
	})

	t.Run("the legacy hook compiles", func(t *testing.T) {
		engine, err := NewEngine(zaptest.NewLogger(t), "", LegacyEnv{ProjectCreatedURL: "http://courier.internal/hook"})
		require.NoError(t, err)
		assert.True(t, engine.Enabled(ProjectCreated.Name))
	})
}

func TestLoadPrecedence(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.yaml")
	require.NoError(t, os.WriteFile(path, []byte("version: v1\n"), 0o600))

	t.Run("a file wins over the legacy variables", func(t *testing.T) {
		cfg, warnings, err := Load(path, LegacyEnv{ProjectCreatedURL: "http://legacy.internal/hook"})
		require.NoError(t, err)
		assert.Empty(t, cfg.Hooks)
		assert.Contains(t, joined(warnings), "are ignored")
	})

	t.Run("neither configured yields no config", func(t *testing.T) {
		cfg, warnings, err := Load("", LegacyEnv{})
		require.NoError(t, err)
		assert.Nil(t, cfg)
		assert.Empty(t, warnings)
	})

	t.Run("a missing file is a boot failure", func(t *testing.T) {
		_, _, err := Load(filepath.Join(dir, "nope.yaml"), LegacyEnv{})
		require.Error(t, err)
	})
}

func TestRegistryRejectsDuplicates(t *testing.T) {
	t.Parallel()

	assert.True(t, IsRegistered("project.created"))
	assert.Contains(t, Registered(), "project.created")
	assert.Panics(t, func() { MustRegister(Definition{Name: "project.created", Version: "v1"}) })
	assert.Panics(t, func() { MustRegister(Definition{Name: "no.version"}) })
}

func joined(warnings []string) string {
	out := ""
	for _, warning := range warnings {
		out += warning + "\n"
	}
	return out
}
