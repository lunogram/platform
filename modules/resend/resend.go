package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/lunogram/platform/pkg/modules/providers"
	"github.com/resend/resend-go/v3"
)

// Exit code convention for WASM provider modules:
//
//	 0  — success
//	-1  — transient/retryable error  (rate limit, network, server error)
//	-2  — permanent/non-retryable error (invalid recipient, validation, auth)
const (
	ExitTransient int32 = -1
	ExitPermanent int32 = -2
	ExitSuccess   int32 = 0
)

// Config holds the Resend provider configuration persisted by the platform.
type Config struct {
	APIKey        string `json:"apiKey"`
	WebhookSecret string `json:"webhookSecret"`
	WebhookID     string `json:"webhookId"`
}

// resendWebhookTag is a single tag entry echoed back on the Resend webhook
// payload. Resend uses tags as its native custom-metadata mechanism and
// echoes any tags set on the original send back on every delivery webhook.
type resendWebhookTag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// resendWebhookPayload is the JSON body from a Resend webhook callback.
type resendWebhookPayload struct {
	Type      string `json:"type"`
	CreatedAt string `json:"created_at"`
	Data      struct {
		EmailID string             `json:"email_id"`
		To      []string           `json:"to"`
		From    string             `json:"from"`
		Subject string             `json:"subject"`
		Tags    []resendWebhookTag `json:"tags"`
	} `json:"data"`
}

// extractInboxMessageID scans the supplied Resend webhook tags for the
// canonical inbox_message_id metadata key and parses it as a UUID. Returns
// the zero UUID and a nil error when the tag is absent (a pre-T06 send had
// no metadata to echo) so that webhook ingestion remains backwards
// compatible. A non-nil error is returned only when the tag is present but
// not a valid UUID; callers should log the error and proceed with the zero
// UUID rather than reject the webhook (provider retries are expensive).
func extractInboxMessageID(tags []resendWebhookTag) (uuid.UUID, error) {
	for _, tag := range tags {
		if tag.Name != providers.MetadataKeyInboxMessageID {
			continue
		}
		if tag.Value == "" {
			return uuid.Nil, nil
		}
		parsed, err := uuid.Parse(tag.Value)
		if err != nil {
			return uuid.Nil, fmt.Errorf("invalid inbox_message_id tag %q: %w", tag.Value, err)
		}
		return parsed, nil
	}
	return uuid.Nil, nil
}

// formatAddress formats an EmailAddress as "Name <addr>" or just "addr".
func formatAddress(address providers.EmailAddress) string {
	if address.Name != "" {
		return fmt.Sprintf("%s <%s>", address.Name, address.Address)
	}
	return address.Address
}

// classifyError inspects a Resend SDK error and returns the appropriate WASM
// exit code: transient (retryable) or permanent.
func classifyError(err error) int32 {
	if err == nil {
		return 0
	}
	if errors.Is(err, resend.ErrRateLimit) {
		return ExitTransient
	}
	// The SDK wraps server errors as plain errors with "[ERROR]: " prefix.
	// We conservatively treat unknown errors as transient so they get retried.
	msg := err.Error()
	if strings.Contains(msg, "validation") ||
		strings.Contains(msg, "missing") ||
		strings.Contains(msg, "invalid") {
		return ExitPermanent
	}
	return ExitTransient
}

// mapWebhookEvent maps a Resend event type string to a canonical
// WebhookEventName. Returns (eventName, ok).
func mapWebhookEvent(eventType string) (providers.WebhookEventName, bool) {
	switch eventType {
	case resend.EventEmailSent:
		return providers.EventSent, true
	case resend.EventEmailDelivered:
		return providers.EventDelivered, true
	case resend.EventEmailOpened:
		return providers.EventOpened, true
	case resend.EventEmailClicked:
		return providers.EventClicked, true
	case resend.EventEmailBounced:
		return providers.EventBounced, true
	case resend.EventEmailComplained:
		return providers.EventComplained, true
	case resend.EventEmailDeliveryDelayed:
		return providers.EventDeferred, true
	default:
		return 0, false
	}
}

// ComposeSendEmailRequest converts platform email payload to a Resend SDK request.
// Any metadata key/value pairs are forwarded as Resend tags so they are echoed
// back on delivery webhooks (Resend tag values must match ^[a-zA-Z0-9_-]+$,
// which UUIDs satisfy).
func ComposeSendEmailRequest(email providers.EmailPayload, metadata map[string]string) *resend.SendEmailRequest {
	req := &resend.SendEmailRequest{
		From:    formatAddress(email.From),
		To:      []string{email.To},
		Subject: email.Subject,
		Html:    email.HTML,
		Text:    email.Text,
		Headers: email.Headers,
	}

	if email.Cc != nil {
		req.Cc = []string{*email.Cc}
	}
	if email.Bcc != nil {
		req.Bcc = []string{*email.Bcc}
	}
	if email.ReplyTo != nil {
		req.ReplyTo = *email.ReplyTo
	}

	if len(metadata) > 0 {
		req.Tags = make([]resend.Tag, 0, len(metadata))
		for name, value := range metadata {
			req.Tags = append(req.Tags, resend.Tag{Name: name, Value: value})
		}
	}

	return req
}

// webhookEvents returns the list of Resend event types the provider subscribes to.
func webhookEvents() []string {
	return []string{
		resend.EventEmailSent,
		resend.EventEmailDelivered,
		resend.EventEmailBounced,
		resend.EventEmailOpened,
		resend.EventEmailClicked,
		resend.EventEmailComplained,
	}
}
