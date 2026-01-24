package channels

import (
	"encoding/json"
	"fmt"

	"github.com/lunogram/platform/pkg/modules/providers"
	"github.com/lunogram/platform/services/nexus/internal/store"
)

// EmailTemplateData represents the structure of email template data.
type EmailTemplateData struct {
	From      providers.EmailAddress `json:"from"`
	Subject   string                 `json:"subject"`
	HTML      string                 `json:"html"`
	Text      string                 `json:"text,omitempty"`
	Preheader string                 `json:"preheader,omitempty"`
	ReplyTo   string                 `json:"reply_to,omitempty"`
	Cc        string                 `json:"cc,omitempty"`
	Bcc       string                 `json:"bcc,omitempty"`
}

func ComposeEmail(config map[string]any, template store.Template, user *store.User) (*providers.SendRequest[map[string]any], error) {
	if user.Email == nil {
		return nil, fmt.Errorf("user has no email address")
	}

	var data EmailTemplateData
	if err := json.Unmarshal(template.Data, &data); err != nil {
		return nil, fmt.Errorf("failed to parse email template data: %w", err)
	}

	payload := providers.EmailPayload{
		To:      *user.Email,
		From:    data.From,
		Subject: data.Subject,
		HTML:    data.HTML,
		Text:    data.Text,
	}

	if data.ReplyTo != "" {
		payload.ReplyTo = &data.ReplyTo
	}

	if data.Cc != "" {
		payload.Cc = &data.Cc
	}

	if data.Bcc != "" {
		payload.Bcc = &data.Bcc
	}

	return providers.NewEmailRequest(config, payload)
}
