package channels

import (
	"encoding/json"
	"fmt"

	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/users"
	"github.com/lunogram/platform/pkg/modules/providers"
)

// SMSTemplateData represents SMS template content.
type SMSTemplateData struct {
	From string `json:"from,omitempty"`
	Body string `json:"body"`
}

func ComposeSMS(config map[string]any, template management.Template, user *users.User) (*providers.SendRequest[map[string]any], error) {
	if user.Phone == nil {
		return nil, fmt.Errorf("user has no phone number")
	}

	var data SMSTemplateData
	if err := json.Unmarshal(template.Data, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal SMS template data: %w", err)
	}

	providerData, _ := config["data"].(map[string]any)
	defaultFrom, _ := providerData["default_from"].(string)
	defaultFromLocked, _ := providerData["default_from_locked"].(bool)

	fromNumber := data.From

	if defaultFromLocked || fromNumber == "" {
		if defaultFrom != "" {
			fromNumber = defaultFrom
		}
	}

	if fromNumber == "" {
		return nil, fmt.Errorf("no from number specified in template or provider config")
	}

	payload := providers.SMSPayload{
		To:   *user.Phone,
		From: fromNumber,
		Body: data.Body,
	}

	return providers.NewSMSRequest(config, payload)
}
