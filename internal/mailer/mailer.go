// Package mailer sends the platform's own transactional mail: account
// verification, password resets and the security notices that go with them.
//
// It is deliberately separate from the project email providers in
// internal/providers. Those are WASM modules scoped to a project and configured
// in the database, which cannot serve authentication mail: the first admin signs
// up before any project exists, so auth mail must not depend on tenant data.
package mailer

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// Message is one transactional email. Both parts are always populated -- a
// text/plain alternative is what keeps the message readable in clients that
// refuse HTML and what keeps it out of spam folders.
type Message struct {
	To      string
	Subject string
	HTML    string
	Text    string

	// ActionURL is the single link the message asks the recipient to follow. It
	// is carried separately from the rendered bodies so [LogMailer] can surface
	// it without parsing HTML; nothing else reads it.
	ActionURL string
}

// Mailer delivers a [Message]. Implementations are safe for concurrent use.
type Mailer interface {
	Send(ctx context.Context, message Message) error
}

// Config configures the platform mailer. With no Host set the platform falls
// back to [LogMailer], which keeps the docker-compose quickstart usable without
// standing up an SMTP server.
type Config struct {
	Host     string `env:"HOST"`
	Port     int    `env:"PORT" envDefault:"587"`
	Username string `env:"USERNAME"`
	Password string `env:"PASSWORD"`

	FromAddress string `env:"FROM_ADDRESS"`
	FromName    string `env:"FROM_NAME" envDefault:"Lunogram"`

	// TLS selects how the connection is secured: "starttls" upgrades a plaintext
	// connection and fails when the server does not offer STARTTLS, "implicit"
	// dials TLS directly (the submission-over-TLS port, usually 465), and "none"
	// sends in the clear. "none" exists for a relay on a trusted local network
	// and nowhere else.
	TLS string `env:"TLS" envDefault:"starttls"`

	// Timeout bounds a single delivery attempt end to end. Sends are dispatched
	// off the request goroutine, so this only bounds the worker.
	Timeout time.Duration `env:"TIMEOUT" envDefault:"30s"`
}

// Configured reports whether an SMTP transport can be built from this
// configuration.
func (c Config) Configured() bool { return c.Host != "" }

const (
	TLSStartTLS = "starttls"
	TLSImplicit = "implicit"
	TLSNone     = "none"
)

// New builds the transport for the given configuration: SMTP when a host is
// configured, and otherwise the development [LogMailer].
func New(config Config, logger *zap.Logger) (Mailer, error) {
	if !config.Configured() {
		return NewLogMailer(logger), nil
	}
	return NewSMTP(config)
}
