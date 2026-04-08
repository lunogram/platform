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

func ComposePush(_ context.Context, config map[string]any, template management.Template, user *subjects.User, devices subjects.Devices) (providers.SendRequest[map[string]any], error) {
	if !user.HasPushDevice {
		return providers.SendRequest[map[string]any]{}, fmt.Errorf("user has no push-enabled device")
	}

	var data PushTemplateData
	if err := json.Unmarshal(template.Data, &data); err != nil {
		return providers.SendRequest[map[string]any]{}, fmt.Errorf("failed to unmarshal push template data: %w", err)
	}

	tokens := make([]string, 0, len(devices))
	apnsTokens := make([]string, 0, len(devices))
	webPushTargets := make([]providers.WebPushTarget, 0, len(devices))

	for _, device := range devices {
		if device.PushConfig == nil {
			continue
		}
		switch device.PushConfig.Type {
		case subjects.PushConfigTypeFCM:
			tokens = append(tokens, device.PushConfig.Token)
		case subjects.PushConfigTypeAPNs:
			apnsTokens = append(apnsTokens, device.PushConfig.Token)
		case subjects.PushConfigTypeWebPush:
			target := providers.WebPushTarget{
				Endpoint: device.PushConfig.Endpoint,
			}
			if device.PushConfig.ExpirationTime != nil {
				exp := device.PushConfig.ExpirationTime.Unix()
				target.ExpirationTime = &exp
			}
			if device.PushConfig.Keys != nil {
				target.Keys.Auth = device.PushConfig.Keys.Auth
				target.Keys.P256dh = device.PushConfig.Keys.P256dh
			}
			webPushTargets = append(webPushTargets, target)
		}
	}

	if len(tokens) == 0 && len(apnsTokens) == 0 && len(webPushTargets) == 0 {
		return providers.SendRequest[map[string]any]{}, fmt.Errorf("user has no devices with push tokens or web push subscriptions")
	}

	custom := data.Data
	if custom == nil {
		custom = make(map[string]any)
	}

	payload := providers.PushPayload{
		Tokens:         tokens,
		APNsTokens:     apnsTokens,
		WebPushTargets: webPushTargets,
		Title:          data.Title,
		Body:           data.Body,
		Data:           custom,
	}

	return providers.NewPushRequest(config, payload)
}
