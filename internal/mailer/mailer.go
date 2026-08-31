// Package mailer sends the platform's own transactional mail: account
// verification, password resets and the security notices that go with them.
//
// It is deliberately separate from the project email providers in
// internal/providers. Those are WASM modules scoped to a project and configured
// in the database, which cannot serve authentication mail: the first admin signs
// up before any project exists, so auth mail must not depend on tenant data.
//
// Mail leaves the platform through one of two channels. SMTP speaks to a relay
// directly. The webhook channel posts the rendered message to an endpoint the
// operator configures, which is how a deployment puts its own system — or an
// HTTP-only provider like Resend or Postmark — in the path without the platform
// growing a client per provider. Both channels carry the same rendered
// [Message], so the templates and the copy do not change when the channel does.
package mailer

import (
	"context"
	"fmt"
	"time"

	"github.com/lunogram/platform/internal/outbound"
	"go.uber.org/zap"
)

// Channels a message can leave the platform through.
const (
	ChannelSMTP    = "smtp"
	ChannelWebhook = "webhook"
)

// Message is one transactional email. Both parts are always populated -- a
// text/plain alternative is what keeps the message readable in clients that
// refuse HTML and what keeps it out of spam folders.
type Message struct {
	To      string
	Subject string
	HTML    string
	Text    string

	// Kind names the message that was rendered (see the Kind constants). The
	// webhook channel exposes it in the template context so a receiver can
	// route or re-render per message without matching on the subject line.
	Kind string

	// ActionURL is the single link the message asks the recipient to follow. It
	// is carried separately from the rendered bodies so a channel can surface it
	// without parsing HTML.
	ActionURL string
}

// Mailer delivers a [Message]. Implementations are safe for concurrent use.
type Mailer interface {
	Send(ctx context.Context, message Message) error
}

// Config configures the platform mailer.
type Config struct {
	// Channel selects how mail leaves the platform: "smtp" or "webhook". There
	// is no default and no fallback that quietly swallows mail: a deployment
	// offering password logins has to say where its mail goes, and one that
	// does not is refused at boot rather than at the first registration.
	Channel string `env:"CHANNEL" yaml:"channel"`

	// ProductName is the name the messages call this deployment.
	ProductName string `env:"PRODUCT_NAME" yaml:"product_name"`

	// Timeout bounds a single delivery attempt end to end. Sends are dispatched
	// off the request goroutine, so this only bounds the worker.
	Timeout time.Duration `env:"TIMEOUT" yaml:"timeout"`

	From      From          `envPrefix:"FROM_" yaml:"from"`
	SMTP      SMTPConfig    `envPrefix:"SMTP_" yaml:"smtp"`
	Webhook   WebhookConfig `envPrefix:"WEBHOOK_" yaml:"webhook"`
	Templates Templates     `envPrefix:"TEMPLATES_" yaml:"templates"`
}

// From is the sender every message is addressed from.
type From struct {
	Address string `env:"ADDRESS" yaml:"address"`
	Name    string `env:"NAME" yaml:"name"`
}

// SMTPConfig configures the SMTP channel.
type SMTPConfig struct {
	Host     string `env:"HOST" yaml:"host"`
	Port     int    `env:"PORT" yaml:"port"`
	Username string `env:"USERNAME" yaml:"username"`
	Password string `env:"PASSWORD" yaml:"password"`

	// TLS selects how the connection is secured: "starttls" upgrades a plaintext
	// connection and fails when the server does not offer STARTTLS, "implicit"
	// dials TLS directly (the submission-over-TLS port, usually 465), and "none"
	// sends in the clear. "none" exists for a relay on a trusted local network
	// -- and for Mailpit in development -- and nowhere else.
	TLS string `env:"TLS" yaml:"tls"`
}

// WebhookConfig configures the webhook channel: where the rendered message is
// posted and how the request body is shaped.
type WebhookConfig struct {
	URL    string `env:"URL" yaml:"url"`
	Method string `env:"METHOD" yaml:"method"`

	// Body is a JSONNet template producing the request body, as a base64://
	// reference, a file:// reference or an inline snippet. When empty the
	// embedded default is used, which produces the shape Mailpit's send API
	// accepts.
	Body string `env:"BODY" yaml:"body"`

	Timeout          time.Duration       `env:"TIMEOUT" yaml:"timeout"`
	Network          outbound.Network    `yaml:"network"`
	Retry            *outbound.Retry     `yaml:"retry"`
	Auth             outbound.AuthConfig `yaml:"auth"`
	MaxResponseBytes int64               `env:"MAX_RESPONSE_BYTES" yaml:"max_response_bytes"`
}

// DefaultConfig returns the mailer settings used when nothing configures them.
//
// Channel is deliberately absent: there is no safe default destination for
// mail, so it stays unset and [New] refuses.
func DefaultConfig() Config {
	return Config{
		ProductName: "Lunogram",
		Timeout:     30 * time.Second,
		From: From{
			Name: "Lunogram",
		},
		SMTP: SMTPConfig{
			Port: 587,
			TLS:  TLSStartTLS,
		},
		Webhook: WebhookConfig{
			Method: "POST",
		},
	}
}

// Configured reports whether a channel has been selected.
func (c Config) Configured() bool { return c.Channel != "" }

const (
	TLSStartTLS = "starttls"
	TLSImplicit = "implicit"
	TLSNone     = "none"
)

// New builds the channel named by the configuration. baseDir is the directory
// the configuration was read from, against which a relative file:// reference
// resolves.
func New(config Config, baseDir string, logger *zap.Logger) (Mailer, error) {
	switch config.Channel {
	case ChannelSMTP:
		return NewSMTP(config)
	case ChannelWebhook:
		return NewWebhook(config, baseDir, logger)
	case "":
		return nil, fmt.Errorf(
			"mailer: no channel is configured. Set mail.channel to %q or %q (MAIL_CHANNEL), "+
				"because a deployment offering password logins cannot confirm an address or reset a password without sending mail",
			ChannelSMTP, ChannelWebhook)
	default:
		return nil, fmt.Errorf("mailer: unknown channel %q, expected %q or %q", config.Channel, ChannelSMTP, ChannelWebhook)
	}
}
