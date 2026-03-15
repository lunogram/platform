package channels

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/lunogram/platform/pkg/modules/providers"
)

// PushTemplateData represents push notification template content.
type PushTemplateData struct {
	Title string         `json:"title"`
	Body  string         `json:"body"`
	Data  map[string]any `json:"data,omitempty"`
}

func ComposePush(_ context.Context, config map[string]any, template management.Template, user *subjects.User, devices subjects.Devices) (*providers.SendRequest[map[string]any], error) {
	if !user.HasPushDevice {
		return nil, fmt.Errorf("user has no push-enabled device")
	}

	var data PushTemplateData
	if err := json.Unmarshal(template.Data, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal push template data: %w", err)
	}

	tokens := make([]string, 0, len(devices))
	for _, device := range devices {
		if device.HasPushToken() {
			tokens = append(tokens, *device.Token)
		}
	}

	if len(tokens) == 0 {
		return nil, fmt.Errorf("user has no devices with push tokens")
	}

	custom := data.Data
	if custom == nil {
		custom = make(map[string]any)
	}

	payload := providers.PushPayload{
		Tokens: tokens,
		Title:  data.Title,
		Body:   data.Body,
		Data:   custom,
	}

	return providers.NewPushRequest(config, payload)
}
