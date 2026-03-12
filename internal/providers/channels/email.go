package channels

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/lunogram/platform/pkg/modules/providers"
)

// EmailFromData represents the from address in email template data.
// Uses "email" field to match frontend convention.
type EmailFromData struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email"`
}

// EmailCodeData holds the React Email source and compiled bundle.
type EmailCodeData struct {
	Source     string `json:"source,omitempty"`
	Bundle     string `json:"bundle,omitempty"`
	BundleHash string `json:"bundle_hash,omitempty"`
}

// EmailPlaintextData holds plain text overrides for a template.
type EmailPlaintextData struct {
	Custom string `json:"custom,omitempty"`
}

// EmailTemplateData represents the structure of email template data.
type EmailTemplateData struct {
	From      EmailFromData      `json:"from"`
	Subject   string             `json:"subject"`
	HTML      string             `json:"html"`
	Text      string             `json:"text,omitempty"`
	Preheader string             `json:"preheader,omitempty"`
	ReplyTo   string             `json:"reply_to,omitempty"`
	Cc        string             `json:"cc,omitempty"`
	Bcc       string             `json:"bcc,omitempty"`
	Code      EmailCodeData      `json:"code,omitempty"`
	Plaintext EmailPlaintextData `json:"plaintext,omitempty"`
}

// ComposeEmail creates a SendRequest for email delivery to a user.
// It extracts the recipient from user.Email.
func ComposeEmail(config map[string]any, template management.Template, user *subjects.User) (*providers.SendRequest[map[string]any], error) {
	if user.Email == nil {
		return nil, fmt.Errorf("user has no email address")
	}

	return ComposePayload(config, template.Data, *user.Email)
}

// ComposeEmailTemplateData takes raw template JSON, and if it contains React Email
// source code, compiles (when no bundle is cached) and renders it with the
// given props. Returns the updated template data as raw JSON ready for use
// with ComposePayload.
func ComposeEmailTemplateData(ctx context.Context, renderer *pubsub.EmailRenderer, projectID uuid.UUID, data json.RawMessage, props map[string]any) (json.RawMessage, error) {
	var email EmailTemplateData
	err := json.Unmarshal(data, &email)
	if err != nil {
		return data, nil
	}

	if email.Code.Source == "" && email.Code.Bundle == "" {
		return data, nil
	}

	if email.Code.Bundle == "" {
		email.Code.Bundle, email.Code.BundleHash, err = renderer.Compile(ctx, projectID, email.Code.Source)
		if err != nil {
			return nil, fmt.Errorf("compile email template: %w", err)
		}
	}

	rendered, err := renderer.Render(ctx, projectID, email.Code.Bundle, props)
	if err != nil {
		return nil, fmt.Errorf("render email template: %w", err)
	}

	email.HTML = rendered.HTML
	email.Text = rendered.PlainText

	// Use custom plain text if provided, otherwise use the
	// auto-generated plain text from the renderer.
	if email.Plaintext.Custom != "" {
		email.Text = email.Plaintext.Custom
	}

	out, err := json.Marshal(email)
	if err != nil {
		return nil, fmt.Errorf("marshal template data: %w", err)
	}

	return out, nil
}

// ComposePayload creates a SendRequest for email delivery to an explicit recipient.
// It parses the template data, resolves from-address defaults from the provider config,
// and builds the final EmailPayload.
func ComposePayload(config map[string]any, templateData json.RawMessage, to string) (*providers.SendRequest[map[string]any], error) {
	var data EmailTemplateData
	if err := json.Unmarshal(templateData, &data); err != nil {
		return nil, fmt.Errorf("failed to parse email template data: %w", err)
	}

	defaultFrom, _ := config[ProviderKeyDefaultFrom].(string)
	defaultFromName, _ := config[ProviderKeyDefaultFromName].(string)
	defaultFromLocked, _ := config[ProviderKeyDefaultFromLocked].(bool)

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
		To: to,
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
