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

// Template document types. The type selects which source the renderer
// compiles; an empty value means React Email, so templates stored before
// this field existed keep working unchanged.
const (
	// TemplateTypeReactEmail compiles the JSX in Code.Source.
	TemplateTypeReactEmail = "react-email"
	// TemplateTypeTemplatical compiles the visual document in Blocks.
	TemplateTypeTemplatical = "templatical"
)

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
	Type      string             `json:"type,omitempty"`
	Code      EmailCodeData      `json:"code,omitempty"`
	Blocks    json.RawMessage    `json:"blocks,omitempty"`
	Plaintext EmailPlaintextData `json:"plaintext,omitempty"`
}

// CompileSource returns the source the renderer should compile for this
// template, which differs by document type: a Templatical template compiles
// its visual document, a React Email one its JSX.
func (d EmailTemplateData) CompileSource() string {
	if d.Type == TemplateTypeTemplatical {
		return string(d.Blocks)
	}
	return d.Code.Source
}

// ComposeEmail creates a SendRequest for email delivery to a user.
// templateSender and providerDefaultSender should be pre-resolved (or nil).
func ComposeEmail(ctx context.Context, templateSender, providerDefaultSender *management.SenderIdentity, config map[string]any, template management.Template, user *subjects.User) (providers.SendRequest[map[string]any], error) {
	if user.Email == nil {
		return providers.SendRequest[map[string]any]{}, fmt.Errorf("user has no email address")
	}

	return ComposeEmailPayload(ctx, templateSender, providerDefaultSender, config, template.Data, *user.Email)
}

// ComposeEmailTemplateData takes raw template JSON, and if it contains React Email
// source code, compiles (when no bundle is cached) and renders it with the
// given props. Returns the updated template data as raw JSON ready for use
// with ComposeEmailPayload.
func ComposeEmailTemplateData(ctx context.Context, renderer *pubsub.EmailRenderer, projectID uuid.UUID, data json.RawMessage, props map[string]any) (json.RawMessage, error) {
	var email EmailTemplateData
	err := json.Unmarshal(data, &email)
	if err != nil {
		return data, nil
	}

	source := email.CompileSource()
	if source == "" && email.Code.Bundle == "" {
		return data, nil
	}

	if email.Code.Bundle == "" {
		email.Code.Bundle, email.Code.BundleHash, err = renderer.Compile(ctx, projectID, source)
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

	// Clear the authoring fields after rendering. Both carry {{ }} sequences the
	// downstream Liquid renderer would incorrectly try to evaluate: JSX/JS
	// inline style objects in Code, and merge tags plus arbitrary text content
	// in the Templatical document. Only the rendered HTML and text should reach
	// it.
	email.Code = EmailCodeData{}
	email.Blocks = nil

	out, err := json.Marshal(email)
	if err != nil {
		return nil, fmt.Errorf("marshal template data: %w", err)
	}

	return out, nil
}

// ComposeEmailPayload creates a SendRequest for email delivery to an explicit recipient.
// It uses pre-resolved sender identities for the template and provider default_from.
func ComposeEmailPayload(ctx context.Context, templateSender, providerDefaultSender *management.SenderIdentity, config map[string]any, templateData json.RawMessage, to string) (providers.SendRequest[map[string]any], error) {
	var data EmailTemplateData
	if err := json.Unmarshal(templateData, &data); err != nil {
		return providers.SendRequest[map[string]any]{}, fmt.Errorf("failed to parse email template data: %w", err)
	}

	// Use the pre-resolved template sender identity.
	var fromAddress, fromName string
	if templateSender != nil {
		fromAddress = templateSender.Address()
		traits := templateSender.TraitsMap()
		if name, _ := traits["name"].(string); name != "" {
			fromName = name
		}
	}

	// Template from.name (which may contain Liquid variables) takes priority
	// over the identity trait name.
	if data.From.Name != "" {
		fromName = data.From.Name
	}

	// Use the pre-resolved provider default_from as fallback.
	if providerDefaultSender != nil {
		if fromAddress == "" {
			fromAddress = providerDefaultSender.Address()
		}
		if fromName == "" {
			traits := providerDefaultSender.TraitsMap()
			if name, _ := traits["name"].(string); name != "" {
				fromName = name
			}
		}
	}

	if fromAddress == "" {
		return providers.SendRequest[map[string]any]{}, fmt.Errorf("no from address specified in template or provider config")
	}

	if data.Subject == "" {
		return providers.SendRequest[map[string]any]{}, fmt.Errorf("email subject is required")
	}

	if data.HTML == "" && data.Text == "" {
		return providers.SendRequest[map[string]any]{}, fmt.Errorf("email body is required: provide either html or text content")
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
