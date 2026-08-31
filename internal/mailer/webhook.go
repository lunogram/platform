package mailer

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/lunogram/platform/internal/outbound"
	"github.com/lunogram/platform/internal/webhook"
	"go.uber.org/zap"
)

// defaultWebhookBody is the template used when the operator configures none.
//
//go:embed templates/webhook.jsonnet
var defaultWebhookBody string

// Webhook delivers mail by posting the rendered message to an operator's
// endpoint.
//
// The platform speaks HTTP to one URL rather than growing a client per mail
// provider: Resend, Postmark, SendGrid, Mailgun and SES all accept a send over
// HTTP, and a JSONNet body template is enough to produce any of their request
// shapes. It also means the guarded transport in internal/outbound -- SSRF
// policy, retries, credential handling -- covers platform mail for free, which
// a bespoke client would have had to reimplement.
type Webhook struct {
	client   *outbound.Client
	template *webhook.Template
	url      string
	method   string
	from     From
}

// webhookContext is the `ctx` a body template is evaluated against.
type webhookContext struct {
	Kind    string         `json:"kind"`
	From    webhookFrom    `json:"from"`
	Message webhookMessage `json:"message"`
}

type webhookFrom struct {
	Address string `json:"address"`
	Name    string `json:"name"`
}

type webhookMessage struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	HTML    string `json:"html"`
	Text    string `json:"text"`
	// ActionURL is the link the message carries, offered separately so a
	// receiver that renders its own mail does not have to dig it out of the
	// body. It is empty for a message that carries no link.
	ActionURL string `json:"action_url"`
}

// NewWebhook builds the webhook channel. baseDir resolves a relative file://
// body reference.
func NewWebhook(config Config, baseDir string, logger *zap.Logger) (*Webhook, error) {
	cfg := config.Webhook

	if cfg.URL == "" {
		return nil, fmt.Errorf("mailer: mail.webhook.url (MAIL_WEBHOOK_URL) is required for the %s channel", ChannelWebhook)
	}
	if config.From.Address == "" {
		return nil, fmt.Errorf("mailer: mail.from.address (MAIL_FROM_ADDRESS) is required for the %s channel", ChannelWebhook)
	}
	if err := outbound.ValidateURL(cfg.URL, cfg.network); err != nil {
		return nil, fmt.Errorf("mail.webhook.url: %w", err)
	}

	body := cfg.Body
	if body == "" {
		body = defaultWebhookBody
	}
	template, err := webhook.ParseTemplate("mail.webhook.body", body, baseDir)
	if err != nil {
		return nil, err
	}

	retry := outbound.Retry{}
	if cfg.Retry != nil {
		retry = *cfg.Retry
	}
	retry = retry.WithDefaults(outbound.DefaultRetry(), cfg.Timeout)

	client, err := outbound.NewClient(outbound.Options{
		Timeout:          cfg.Timeout,
		Network:          cfg.network,
		Auth:             cfg.Auth,
		Retry:            retry,
		MaxResponseBytes: cfg.MaxResponseBytes,
		Logger:           logger,
	})
	if err != nil {
		return nil, fmt.Errorf("mail.webhook: %w", err)
	}

	method := cfg.Method
	if method == "" {
		method = http.MethodPost
	}

	return &Webhook{
		client:   client,
		template: template,
		url:      cfg.URL,
		method:   method,
		from:     config.From,
	}, nil
}

func (w *Webhook) Send(ctx context.Context, message Message) error {
	contextJSON, err := json.Marshal(webhookContext{
		Kind: message.Kind,
		From: webhookFrom{Address: w.from.Address, Name: w.from.Name},
		Message: webhookMessage{
			To:        message.To,
			Subject:   message.Subject,
			HTML:      message.HTML,
			Text:      message.Text,
			ActionURL: message.ActionURL,
		},
	})
	if err != nil {
		return fmt.Errorf("mailer: marshal webhook context: %w", err)
	}

	body, err := w.template.Render(contextJSON)
	if err != nil {
		return fmt.Errorf("mailer: %w", err)
	}

	header := http.Header{}
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")

	if _, err := w.client.Do(ctx, outbound.Request{
		Method: w.method,
		URL:    w.url,
		Body:   body,
		Header: header,
	}); err != nil {
		return fmt.Errorf("mailer: webhook delivery failed: %w", err)
	}

	return nil
}
