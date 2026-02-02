package channels

import (
	"encoding/json"
	"fmt"

	"github.com/lunogram/platform/pkg/modules/providers"
	"github.com/lunogram/platform/internal/store"
)

// SMSTemplateData represents SMS template content.
type SMSTemplateData struct {
	Body string `json:"body"`
}

func ComposeSMS(config map[string]any, template store.Template, user *store.User) (*providers.SendRequest[map[string]any], error) {
	if user.Phone == nil {
		return nil, fmt.Errorf("user has no phone number")
	}

	var data SMSTemplateData
	if err := json.Unmarshal(template.Data, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal SMS template data: %w", err)
	}

	payload := providers.SMSPayload{
		To:   *user.Phone,
		Body: data.Body,
	}

	return providers.NewSMSRequest(config, payload)
}
