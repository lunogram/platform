package main

import (
	"errors"
	"fmt"
	"strings"

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

// resendWebhookPayload is the JSON body from a Resend webhook callback.
type resendWebhookPayload struct {
	Type      string `json:"type"`
	CreatedAt string `json:"created_at"`
	Data      struct {
		EmailID string   `json:"email_id"`
		To      []string `json:"to"`
		From    string   `json:"from"`
		Subject string   `json:"subject"`
	} `json:"data"`
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
func ComposeSendEmailRequest(email providers.EmailPayload) *resend.SendEmailRequest {
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
