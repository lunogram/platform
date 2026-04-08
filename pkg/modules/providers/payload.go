package providers

// EmailPayload contains email-specific message data.
type EmailPayload struct {
	To      string            `json:"to"`
	From    EmailAddress      `json:"from"`
	Cc      *string           `json:"cc,omitempty"`
	Bcc     *string           `json:"bcc,omitempty"`
	ReplyTo *string           `json:"reply_to,omitempty"`
	Subject string            `json:"subject"`
	Text    string            `json:"text"`
	HTML    string            `json:"html"`
	Headers map[string]string `json:"headers,omitempty"`
	List    *ListUnsubscribe  `json:"list,omitempty"`
}

// EmailAddress represents an email address with an optional display name.
type EmailAddress struct {
	Name    string `json:"name,omitempty"`
	Address string `json:"address"`
}

// ListUnsubscribe contains list-unsubscribe header information.
type ListUnsubscribe struct {
	Unsubscribe string `json:"unsubscribe"`
}

// SMSPayload contains SMS-specific message data.
type SMSPayload struct {
	To        string   `json:"to"`
	From      string   `json:"from"`
	Body      string   `json:"body"`
	MediaURLs []string `json:"media_urls,omitempty"`
}

// WebPushTarget contains Web Push subscription data for a device.
type WebPushTarget struct {
	Endpoint       string `json:"endpoint"`
	ExpirationTime *int64 `json:"expiration_time,omitempty"`
	Keys           struct {
		Auth   string `json:"auth"`
		P256dh string `json:"p256dh"`
	} `json:"keys"`
}

// PushPayload contains push notification-specific message data.
type PushPayload struct {
	Tokens         []string        `json:"tokens"`
	APNsTokens     []string        `json:"apns_tokens,omitempty"`
	WebPushTargets []WebPushTarget `json:"web_push_targets,omitempty"`
	Title          string          `json:"title"`
	Body           string          `json:"body"`
	ImageURL       *string         `json:"image_url,omitempty"`
	Data           map[string]any  `json:"data,omitempty"`
	Sound          *string         `json:"sound,omitempty"`
	Badge          *int            `json:"badge,omitempty"`
}

// WebhookPayload contains webhook-specific request data.
type WebhookPayload struct {
	Method   string            `json:"method"`
	Endpoint string            `json:"endpoint"`
	Body     map[string]any    `json:"body,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	CacheKey string            `json:"cache_key,omitempty"`
}
