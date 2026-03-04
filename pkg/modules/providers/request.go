package providers

import "encoding/json"

// SendRequest is the input to the provider's send() function.
// The Payload field contains channel-specific data.
type SendRequest[T any] struct {
	Channel Channel         `json:"channel"`
	Config  T               `json:"config"`
	Payload json.RawMessage `json:"payload"`
}

// SendResponse is the output from the provider's send() function.
type SendResponse struct {
	ID       string         `json:"id,omitempty"`
	Status   string         `json:"status"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Helper methods to create channel-specific requests

// NewEmailRequest creates a SendRequest for email.
func NewEmailRequest[T any](config T, payload EmailPayload) (*SendRequest[T], error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &SendRequest[T]{
		Channel: ChannelEmail,
		Config:  config,
		Payload: data,
	}, nil
}

// NewSMSRequest creates a SendRequest for SMS.
func NewSMSRequest[T any](config T, payload SMSPayload) (*SendRequest[T], error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &SendRequest[T]{
		Channel: ChannelSMS,
		Config:  config,
		Payload: data,
	}, nil
}

// NewPushRequest creates a SendRequest for push notifications.
func NewPushRequest[T any](config T, payload PushPayload) (*SendRequest[T], error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &SendRequest[T]{
		Channel: ChannelPush,
		Config:  config,
		Payload: data,
	}, nil
}

// NewWebhookRequest creates a SendRequest for webhooks.
func NewWebhookRequest[T any](config T, payload WebhookPayload) (*SendRequest[T], error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &SendRequest[T]{
		Channel: ChannelWebhook,
		Config:  config,
		Payload: data,
	}, nil
}

// Helper methods to unmarshal channel-specific payloads

// GetEmailPayload unmarshals the payload as EmailPayload.
func (r *SendRequest[T]) GetEmailPayload() (*EmailPayload, error) {
	var payload EmailPayload
	if err := json.Unmarshal(r.Payload, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

// GetSMSPayload unmarshals the payload as SMSPayload.
func (r *SendRequest[T]) GetSMSPayload() (*SMSPayload, error) {
	var payload SMSPayload
	if err := json.Unmarshal(r.Payload, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

// GetPushPayload unmarshals the payload as PushPayload.
func (r *SendRequest[T]) GetPushPayload() (*PushPayload, error) {
	var payload PushPayload
	if err := json.Unmarshal(r.Payload, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

// GetWebhookPayload unmarshals the payload as WebhookPayload.
func (r *SendRequest[T]) GetWebhookPayload() (*WebhookPayload, error) {
	var payload WebhookPayload
	if err := json.Unmarshal(r.Payload, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}
