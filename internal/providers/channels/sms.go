package channels

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/lunogram/platform/pkg/modules/providers"
)

// SMSTemplateData represents SMS template content.
type SMSTemplateData struct {
	From string `json:"from,omitempty"`
	Body string `json:"body"`
}

// ComposeSMS creates a SendRequest for SMS delivery to a user.
// templateSender and providerDefaultSender should be pre-resolved (or nil).
func ComposeSMS(ctx context.Context, templateSender, providerDefaultSender *management.SenderIdentity, config map[string]any, template management.Template, user *subjects.User) (*providers.SendRequest[map[string]any], error) {
	if user.Phone == nil {
		return nil, fmt.Errorf("user has no phone number")
	}

	return ComposeSMSPayload(ctx, templateSender, providerDefaultSender, config, template.Data, *user.Phone)
}

// ComposeSMSPayload creates a SendRequest for SMS delivery to an explicit recipient.
// It uses pre-resolved sender identities for the template and provider default_from.
func ComposeSMSPayload(ctx context.Context, templateSender, providerDefaultSender *management.SenderIdentity, config map[string]any, templateData json.RawMessage, to string) (*providers.SendRequest[map[string]any], error) {
	var data SMSTemplateData
	if err := json.Unmarshal(templateData, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal SMS template data: %w", err)
	}

	// Use the pre-resolved template sender identity.
	var fromNumber string
	if templateSender != nil {
		fromNumber = templateSender.Address()
	}

	// Fall back to provider default_from.
	if fromNumber == "" && providerDefaultSender != nil {
		fromNumber = providerDefaultSender.Address()
	}

	if fromNumber == "" {
		return nil, fmt.Errorf("no from number specified in template or provider config")
	}

	payload := providers.SMSPayload{
		To:   to,
		From: fromNumber,
		Body: data.Body,
	}

	return providers.NewSMSRequest(config, payload)
}
