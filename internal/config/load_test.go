package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
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
    password: ${SMTP_PASSWORD}
`))

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "s3cr3t", cfg.Mail.SMTP.Password)
}

// Expansion happens on the parsed document, so a secret is free to contain
// whatever a secret contains -- no quoting ritual, and nothing in the value
// can reach the rest of the file.
func TestLoadCarriesAwkwardSecrets(t *testing.T) {
	const secret = "pa#ss: word" + "\n" + "rate_limit:" + "\n" + "  per_minute: 1"
	t.Setenv("SMTP_PASSWORD", secret)
	t.Setenv(ConfigFileEnv, writeConfig(t, `
rate_limit:
  per_minute: 600
mail:
  channel: smtp
  smtp:
    password: ${SMTP_PASSWORD}
`))

	cfg, err := Load()
	require.NoError(t, err)
	assert.Equal(t, secret, cfg.Mail.SMTP.Password)
	assert.Equal(t, 600, cfg.RateLimit.PerMinute, "the value must not reach the rest of the document")
}

// A template is often the one value large enough to want a file or a base64
// payload, but nothing stops it being written inline any more.
func TestLoadCarriesAMultilineTemplate(t *testing.T) {
	const layout = "<html>\n<body>{{ .Heading }}</body>\n</html>"
	t.Setenv("VERIFY_HTML", layout)
	t.Setenv(ConfigFileEnv, writeConfig(t, `
mail:
  channel: smtp
  templates:
    layout:
      html: ${VERIFY_HTML}
`))

	cfg, err := Load()
	require.NoError(t, err)
	assert.Contains(t, cfg.Mail.Templates.Layout.HTML, "{{ .Heading }}")
	assert.Equal(t, layout, cfg.Mail.Templates.Layout.HTML)
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

// base64://${NAME} is the documented way to supply a template from the
// environment, and it is a composition of three separate mechanisms --
// expansion, reference resolution and template parsing. This covers the whole
// chain, because each of them is tested in isolation elsewhere and none of
// those tests would notice the seams coming apart.
func TestBase64TemplateFromTheEnvironment(t *testing.T) {
	const layout = "<html>\n  <body>\n    <h1>{{ .Heading }}</h1>\n    <a href=\"{{ .ActionURL }}\">{{ .ActionLabel }}</a>\n  </body>\n</html>\n"

	t.Setenv("VERIFY_LAYOUT_HTML", base64.StdEncoding.EncodeToString([]byte(layout)))
	t.Setenv(ConfigFileEnv, writeConfig(t, `
mail:
  channel: smtp
  from:
    address: no-reply@example.com
  smtp:
    host: smtp.example.com
  templates:
    layout:
      html: base64://${VERIFY_LAYOUT_HTML}
`))

	cfg, err := Load()
	require.NoError(t, err)

	renderer, err := mailer.NewRenderer(cfg.Mail, "https://console.example.com", cfg.BaseDir())
	require.NoError(t, err)

	message := renderer.VerifyEmail("admin@example.com", "tok", time.Hour)
	assert.Contains(t, message.HTML, "<h1>Confirm your email address</h1>")
	assert.Contains(t, message.HTML, "https://console.example.com/verify-email?token=tok")
	assert.NotContains(t, message.HTML, "base64://", "the payload must be decoded, not passed through")
}

// GNU base64 wraps at 76 columns unless told not to, and an operator setting
// the variable from a shell will hit that.
func TestBase64TemplateSurvivesWrapping(t *testing.T) {
	const subject = "Confirm your address at Example"

	encoded := base64.StdEncoding.EncodeToString([]byte(subject))
	var wrapped strings.Builder
	for i := 0; i < len(encoded); i += 76 {
		wrapped.WriteString(encoded[i:min(i+76, len(encoded))])
		wrapped.WriteString("\n")
	}

	t.Setenv("VERIFY_SUBJECT", wrapped.String())
	t.Setenv(ConfigFileEnv, writeConfig(t, `
mail:
  channel: smtp
  from:
    address: no-reply@example.com
  smtp:
    host: smtp.example.com
  templates:
    verify_email:
      subject: base64://${VERIFY_SUBJECT}
`))

	cfg, err := Load()
	require.NoError(t, err)

	renderer, err := mailer.NewRenderer(cfg.Mail, "https://console.example.com", cfg.BaseDir())
	require.NoError(t, err)

	assert.Equal(t, subject, renderer.VerifyEmail("admin@example.com", "tok", time.Hour).Subject)
}
