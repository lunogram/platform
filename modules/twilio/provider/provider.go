// Package provider contains pure logic functions for the Twilio provider
// module. It is free of WASM (Extism PDK) and Twilio SDK dependencies so
// that it can be tested with standard `go test`.
package provider

import (
	"fmt"
	"net/url"

	"github.com/google/uuid"
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

// Twilio REST API error codes that carry a canonical failure meaning.
// See https://www.twilio.com/docs/api/errors.
const (
	ErrorCodeTooManyRequests    = 20429
	ErrorCodeInvalidToNumber    = 21211
	ErrorCodeFromNotSMSCapable  = 21606
	ErrorCodeOptedOut           = 21610
	ErrorCodeToNotMobile        = 21614
	ErrorCodeTollFreeUnverified = 30032
	ErrorCodeSenderUnregistered = 30034
)

// ClassifyErrorCode maps a Twilio API failure onto a canonical failure reason.
// twilioCode is the numeric `code` field of the Twilio error body (0 when the
// response carried none) and httpStatus is the HTTP status of the response
// (0 when the request never reached Twilio).
func ClassifyErrorCode(httpStatus, twilioCode int) providers.FailureReason {
	switch twilioCode {
	case ErrorCodeOptedOut:
		return providers.ReasonOptedOut
	case ErrorCodeInvalidToNumber, ErrorCodeToNotMobile:
		return providers.ReasonInvalidNumber
	case ErrorCodeFromNotSMSCapable, ErrorCodeTollFreeUnverified, ErrorCodeSenderUnregistered:
		return providers.ReasonUnregistered
	case ErrorCodeTooManyRequests:
		return providers.ReasonRateLimited
	}

	if httpStatus == 429 {
		return providers.ReasonRateLimited
	}

	return providers.ReasonUnknown
}

// ClassifySendError returns the canonical failure reason for a Twilio send
// failure together with the WASM exit code that preserves retry semantics: a
// recipient who opted out must never be retried, a throttled request always
// must. Unclassified failures fall back to ClassifyHTTPStatus.
func ClassifySendError(httpStatus, twilioCode int) (providers.FailureReason, int32) {
	reason := ClassifyErrorCode(httpStatus, twilioCode)

	switch reason {
	case providers.ReasonOptedOut, providers.ReasonInvalidNumber, providers.ReasonUnregistered:
		return reason, ExitPermanent
	case providers.ReasonRateLimited:
		return reason, ExitTransient
	}

	if httpStatus > 0 {
		return reason, ClassifyHTTPStatus(httpStatus)
	}

	return reason, ExitTransient
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

// ExtractInboxMessageID reads the platform inbox-message UUID from the
// `inbox_message_id` query parameter on the Twilio StatusCallback URL.
// AppendStatusCallbackMetadata sets it on the outbound StatusCallback URL,
// and Twilio echoes the full URL back when posting status callbacks.
//
// Returns the zero UUID and a nil error when the query parameter is absent
// (pre-T06 sends did not propagate metadata) so that webhook ingestion
// remains backwards compatible. A non-nil error is returned when the
// parameter is present but not a valid UUID, or when the raw URL fails to
// parse; callers should log the error and proceed with the zero UUID rather
// than reject the webhook.
func ExtractInboxMessageID(rawURL string) (uuid.UUID, error) {
	if rawURL == "" {
		return uuid.Nil, nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse webhook URL: %w", err)
	}
	value := parsed.Query().Get(providers.MetadataKeyInboxMessageID)
	if value == "" {
		return uuid.Nil, nil
	}
	id, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid inbox_message_id query param %q: %w", value, err)
	}
	return id, nil
}

// AppendStatusCallbackMetadata appends the supplied metadata as query
// parameters on the configured Twilio StatusCallback URL. Twilio echoes any
// query string on the callback URL back to the platform when the message
// status changes, so this is how SMS providers (which lack native custom
// metadata) carry the inbox-message UUID to the webhook handler.
//
// If callbackURL is empty or metadata is empty the URL is returned unchanged.
// Existing query parameters are preserved; metadata keys overwrite duplicates.
// If callbackURL fails to parse it is returned unchanged so the caller still
// passes Twilio's own URL validation.
func AppendStatusCallbackMetadata(callbackURL string, metadata map[string]string) string {
	if callbackURL == "" || len(metadata) == 0 {
		return callbackURL
	}
	parsed, err := url.Parse(callbackURL)
	if err != nil {
		return callbackURL
	}
	q := parsed.Query()
	for k, v := range metadata {
		q.Set(k, v)
	}
	parsed.RawQuery = q.Encode()
	return parsed.String()
}
