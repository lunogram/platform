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

// PushPayload contains push notification-specific message data.
type PushPayload struct {
	Tokens   []string       `json:"tokens"`
	Title    string         `json:"title"`
	Body     string         `json:"body"`
	ImageURL *string        `json:"image_url,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
	Sound    *string        `json:"sound,omitempty"`
	Badge    *int           `json:"badge,omitempty"`
}
