package channels

import (
	"encoding/json"
	"fmt"

	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/users"
	"github.com/lunogram/platform/pkg/modules/providers"
)

// EmailFromData represents the from address in email template data.
// Uses "email" field to match frontend convention.
type EmailFromData struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email"`
}

// EmailTemplateData represents the structure of email template data.
type EmailTemplateData struct {
	From      EmailFromData `json:"from"`
	Subject   string        `json:"subject"`
	HTML      string        `json:"html"`
	Text      string        `json:"text,omitempty"`
	Preheader string        `json:"preheader,omitempty"`
	ReplyTo   string        `json:"reply_to,omitempty"`
	Cc        string        `json:"cc,omitempty"`
	Bcc       string        `json:"bcc,omitempty"`
}

func ComposeEmail(config map[string]any, template management.Template, user *users.User) (*providers.SendRequest[map[string]any], error) {
	if user.Email == nil {
		return nil, fmt.Errorf("user has no email address")
	}

	var data EmailTemplateData
	if err := json.Unmarshal(template.Data, &data); err != nil {
		return nil, fmt.Errorf("failed to parse email template data: %w", err)
	}

	providerData, _ := config["data"].(map[string]any)
	defaultFrom, _ := providerData[ProviderKeyDefaultFrom].(string)
	defaultFromName, _ := providerData[ProviderKeyDefaultFromName].(string)
	defaultFromLocked, _ := providerData[ProviderKeyDefaultFromLocked].(bool)

	fromAddress := data.From.Email
	fromName := data.From.Name

	if defaultFromLocked || fromAddress == "" {
		if defaultFrom != "" {
			fromAddress = defaultFrom
		}
	}

	if defaultFromLocked || fromName == "" {
		if defaultFromName != "" {
			fromName = defaultFromName
		}
	}

	if fromAddress == "" {
		return nil, fmt.Errorf("no from address specified in template or provider config")
	}

	payload := providers.EmailPayload{
		To: *user.Email,
		From: providers.EmailAddress{
			Name:    fromName,
			Address: fromAddress,
		},
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
