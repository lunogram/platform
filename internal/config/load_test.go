package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lunogram/platform/internal/mailer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "lunogram.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func TestLoadWithoutAFileIsDefaults(t *testing.T) {
	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, Defaults(), cfg)
}

// The file is an overlay, not a replacement: a document that sets one field
// leaves every other default where it was.
func TestLoadFileOverlaysDefaults(t *testing.T) {
	t.Setenv(ConfigFileEnv, writeConfig(t, `
public_url: https://console.example.com
auth:
  drivers: [password]
  password:
    registration: open
`))

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "https://console.example.com", cfg.PublicURL)
	assert.Equal(t, []string{"password"}, cfg.Auth.Drivers)
	assert.Equal(t, RegistrationOpen, cfg.Auth.Password.Registration)

	assert.Equal(t, ":8080", cfg.HTTPAddress, "an untouched default survives the overlay")
	assert.Equal(t, 600, cfg.RateLimit.PerMinute)
	assert.Equal(t, 8*time.Hour, cfg.Auth.Console.IdleTTL)
}

// The whole reason the defaults are a constructor rather than envDefault tags:
// with tags, this assertion fails, because env.Parse writes the tag default over
// the file whenever the variable is unset.
func TestLoadFileBeatsDefaultsForEveryKind(t *testing.T) {
	t.Setenv(ConfigFileEnv, writeConfig(t, `
database_migrate: false
rate_limit:
  per_minute: 42
auth:
  console:
    idle_ttl: 15m
storage:
  type: s3
`))

	cfg, err := Load()
	require.NoError(t, err)

	assert.False(t, cfg.DatabaseMigrate, "an explicit false is not an absent key")
	assert.Equal(t, 42, cfg.RateLimit.PerMinute)
	assert.Equal(t, 15*time.Minute, cfg.Auth.Console.IdleTTL)
	assert.Equal(t, "s3", cfg.Storage.Type)
}

func TestEnvironmentBeatsFile(t *testing.T) {
	t.Setenv(ConfigFileEnv, writeConfig(t, `
public_url: https://from-the-file.example.com
rate_limit:
  per_minute: 42
`))
	t.Setenv("PUBLIC_URL", "https://from-the-environment.example.com")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "https://from-the-environment.example.com", cfg.PublicURL)
	assert.Equal(t, 42, cfg.RateLimit.PerMinute, "the file still supplies what the environment does not")
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	t.Setenv(ConfigFileEnv, writeConfig(t, "pubic_url: https://typo.example.com\n"))

	_, err := Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pubic_url")
}

func TestLoadExpandsEnvironmentReferences(t *testing.T) {
	t.Setenv("SMTP_PASSWORD", "s3cr3t")
	t.Setenv(ConfigFileEnv, writeConfig(t, `
mail:
  channel: smtp
  smtp:
    host: smtp.example.com
    password: "${SMTP_PASSWORD}"
`))

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "s3cr3t", cfg.Mail.SMTP.Password)
}

func TestLoadReportsMissingFile(t *testing.T) {
	t.Setenv(ConfigFileEnv, filepath.Join(t.TempDir(), "absent.yaml"))

	_, err := Load()
	require.Error(t, err)
}

// Relative file:// references resolve against the configuration's own
// directory, so a config and the templates it points at travel together.
func TestBaseDirIsTheConfigurationsDirectory(t *testing.T) {
	path := writeConfig(t, "public_url: https://console.example.com\n")
	t.Setenv(ConfigFileEnv, path)

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, filepath.Dir(path), cfg.BaseDir())
}

func TestInlineWebhookSectionIsParsed(t *testing.T) {
	t.Setenv(ConfigFileEnv, writeConfig(t, `
webhook:
  outbound:
    version: v1
    hooks:
      project.created:
        - id: provisioning
          url: https://receiver.example.com/hooks
`))

	cfg, err := Load()
	require.NoError(t, err)
	require.NotNil(t, cfg.Webhook.Outbound)
	assert.Len(t, cfg.Webhook.Outbound.Hooks["project.created"], 1)
}

func TestMailTemplatesAreConfigurable(t *testing.T) {
	t.Setenv(ConfigFileEnv, writeConfig(t, `
mail:
  channel: smtp
  templates:
    verify_email:
      subject: Please confirm your address
`))

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, mailer.ChannelSMTP, cfg.Mail.Channel)
	assert.Equal(t, "Please confirm your address", cfg.Mail.Templates.VerifyEmail.Subject)
}

// The shipped example is the first thing an operator copies, so it has to
// actually parse -- including under KnownFields, which is what catches a key
// that was renamed in Go and left behind in the example.
func TestShippedExampleLoads(t *testing.T) {
	t.Setenv("SMTP_PASSWORD", "s3cr3t")
	t.Setenv("PROVISIONING_TOKEN", "tok")
	t.Setenv(ConfigFileEnv, filepath.Join("..", "..", "etc", "lunogram.example.yaml"))

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, mailer.ChannelSMTP, cfg.Mail.Channel)
	assert.Equal(t, "s3cr3t", cfg.Mail.SMTP.Password)
	assert.Equal(t, []string{"password"}, cfg.Auth.Drivers)
	require.NotNil(t, cfg.Webhook.Outbound)
	assert.Len(t, cfg.Webhook.Outbound.Hooks["project.created"], 1)
}
