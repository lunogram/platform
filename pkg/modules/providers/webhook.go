package providers

import "encoding/json"

// WebhookRequest is the input to the provider's webhook() function.
// It contains the raw HTTP request data and the provider's configuration
// so the WASM module can verify signatures and parse the payload.
type WebhookRequest struct {
	// Config is the provider's configuration JSON (same shape as send config,
	// includes signing secrets needed for webhook verification).
	Config json.RawMessage `json:"config"`

	// Headers contains the HTTP request headers with lowercased keys.
	Headers map[string]string `json:"headers"`

	// Body is the raw HTTP request body.
	Body json.RawMessage `json:"body"`

	// URL is the full request URL including scheme, host, path, and query string.
	// Some providers (e.g., Twilio) include the callback URL in their HMAC signature.
	URL string `json:"url"`
}

// WebhookResponse is the output from the provider's webhook() function.
type WebhookResponse struct {
	Events []WebhookEvent `json:"events"`
}

//go:generate go run golang.org/x/tools/cmd/stringer@latest -type=WebhookEventName -linecomment

// WebhookEventName is a constrained type representing a canonical provider
// webhook event. WASM modules MUST map provider-specific event types to one
// of these canonical names so that downstream consumers (metrics, campaign
// state machines, journey triggers) can rely on a stable, finite set.
type WebhookEventName int

const (
	// EventSent indicates the provider accepted the message and began delivery.
	EventSent WebhookEventName = iota + 1 // provider.sent

	// EventDelivered indicates the message was successfully delivered to the recipient.
	EventDelivered // provider.delivered

	// EventBounced indicates the message could not be delivered (hard or soft bounce).
	EventBounced // provider.bounced

	// EventDeferred indicates the provider temporarily delayed delivery and will retry.
	EventDeferred // provider.deferred

	// EventDropped indicates the provider dropped the message before attempting delivery
	// (e.g., previous hard bounce, unsubscribed recipient, spam report).
	EventDropped // provider.dropped

	// EventOpened indicates the recipient opened the message (pixel-tracked).
	EventOpened // provider.opened

	// EventClicked indicates the recipient clicked a link in the message.
	EventClicked // provider.clicked

	// EventComplained indicates the recipient marked the message as spam.
	EventComplained // provider.complained

	// EventUnsubscribed indicates the recipient unsubscribed via a provider-managed link.
	EventUnsubscribed // provider.unsubscribed

	webhookEventNameCount // sentinel
)

// WebhookEvent represents a single delivery event extracted from a provider webhook payload.
// A single webhook POST from a provider may contain multiple events (e.g., SendGrid batches).
type WebhookEvent struct {
	// EventName is the canonical event name that will be published to NATS.
	// Must be one of the WebhookEventName constants (e.g. EventDelivered, EventBounced).
	EventName WebhookEventName `json:"event_name"`

	// MessageID is the provider's message ID that was returned in SendResponse.ID.
	// Used to correlate this event back to the original send.
	MessageID string `json:"message_id"`

	// Timestamp is when the event occurred according to the provider.
	// ISO 8601 format. Falls back to receipt time if not available.
	Timestamp string `json:"timestamp,omitempty"`

	// Data contains provider-specific event metadata.
	// This becomes the user event's data payload.
	Data map[string]any `json:"data,omitempty"`
}
