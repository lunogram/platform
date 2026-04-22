// Package provider contains pure logic functions for the Twilio provider
// module. It is free of WASM (Extism PDK) and Twilio SDK dependencies so
// that it can be tested with standard `go test`.
package provider

import (
	"net/url"

	"github.com/lunogram/platform/pkg/modules/providers"
)

// Exit code convention for WASM provider modules:
//
//	 0  — success
//	-1  — transient/retryable error  (rate limit, network, server error)
//	-2  — permanent/non-retryable error (invalid recipient, validation, auth)
const (
	ExitSuccess   int32 = 0
	ExitTransient int32 = -1
	ExitPermanent int32 = -2
)

// Config holds the Twilio provider configuration persisted by the platform.
type Config struct {
	AccountSID string `json:"accountSid"`
	AuthToken  string `json:"authToken"`
	WebhookURL string `json:"webhookUrl"`
}

// WebhookPayload represents the form-encoded fields Twilio POSTs
// to the StatusCallback URL when a message status changes.
type WebhookPayload struct {
	MessageSid    string
	MessageStatus string
	To            string
	From          string
	AccountSid    string
	ApiVersion    string
	ErrorCode     string
	ErrorMessage  string
}

// ClassifyHTTPStatus returns the appropriate WASM exit code for an HTTP status
// code returned by the Twilio API. This is called by the main module after
// extracting the status from a twilioclient.TwilioRestError.
func ClassifyHTTPStatus(status int) int32 {
	// 429 rate-limit is retryable.
	if status == 429 {
		return ExitTransient
	}
	// Other 4xx errors are permanent (bad request, auth, not found, etc.).
	if status >= 400 && status < 500 {
		return ExitPermanent
	}
	// 5xx errors are transient (server-side, retryable).
	return ExitTransient
}

// MapWebhookStatus maps a Twilio message status string to a canonical
// WebhookEventName. Returns (eventName, ok).
func MapWebhookStatus(status string) (providers.WebhookEventName, bool) {
	switch status {
	case "sent":
		return providers.EventSent, true
	case "delivered", "read":
		return providers.EventDelivered, true
	case "failed", "undelivered":
		return providers.EventBounced, true
	case "queued", "accepted", "sending":
		return providers.EventDeferred, true
	default:
		return 0, false
	}
}

// ParseWebhookBody parses a URL-encoded Twilio webhook callback body into
// a WebhookPayload struct.
func ParseWebhookBody(body []byte) (WebhookPayload, error) {
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return WebhookPayload{}, err
	}

	return WebhookPayload{
		MessageSid:    values.Get("MessageSid"),
		MessageStatus: values.Get("MessageStatus"),
		To:            values.Get("To"),
		From:          values.Get("From"),
		AccountSid:    values.Get("AccountSid"),
		ApiVersion:    values.Get("ApiVersion"),
		ErrorCode:     values.Get("ErrorCode"),
		ErrorMessage:  values.Get("ErrorMessage"),
	}, nil
}

// ParseWebhookParams converts the raw body bytes into a map[string]string
// suitable for Twilio signature validation.
func ParseWebhookParams(body []byte) (map[string]string, error) {
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, err
	}

	params := make(map[string]string, len(values))
	for k, v := range values {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	return params, nil
}

// ValidateConfig checks that required configuration fields are present and
// returns a map of field name → error message for any that are missing.
func ValidateConfig(config Config) map[string]string {
	errs := make(map[string]string)
	if config.AccountSID == "" {
		errs["accountSid"] = "Account SID is required"
	}
	if config.AuthToken == "" {
		errs["authToken"] = "Auth Token is required"
	}
	return errs
}

// ResolveSender returns the sender phone number from the payload.
// The platform's sender identity system is responsible for resolving the
// correct "from" number before the request reaches the plugin, so the
// payload should always contain a non-empty value.
func ResolveSender(payloadFrom string) (string, bool) {
	if payloadFrom != "" {
		return payloadFrom, true
	}
	return "", false
}
