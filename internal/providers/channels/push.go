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

	// Collect FCM tokens
	tokens := make([]string, 0, len(devices))
	for _, device := range devices {
		if device.HasFCMToken() {
			tokens = append(tokens, *device.Token)
		}
	}

	// Collect Web Push subscriptions
	webPushTargets := make([]providers.WebPushTarget, 0, len(devices))
	for _, device := range devices {
		if device.HasWebPushSubscription() {
			target := providers.WebPushTarget{
				Endpoint: device.DeviceCredentials.Endpoint,
			}
			if device.DeviceCredentials.ExpirationTime != nil {
				expTime := device.DeviceCredentials.ExpirationTime.Unix()
				target.ExpirationTime = &expTime
			}
			target.Keys.Auth = device.DeviceCredentials.Keys.Auth
			target.Keys.P256dh = device.DeviceCredentials.Keys.P256dh
			webPushTargets = append(webPushTargets, target)
		}
	}

	// Ensure we have at least one target
	if len(tokens) == 0 && len(webPushTargets) == 0 {
		return nil, fmt.Errorf("user has no devices with push tokens or web push subscriptions")
	}

	custom := data.Data
	if custom == nil {
		custom = make(map[string]any)
	}

	payload := providers.PushPayload{
		Tokens:         tokens,
		WebPushTargets: webPushTargets,
		Title:          data.Title,
		Body:           data.Body,
		Data:           custom,
	}

	return providers.NewPushRequest(config, payload)
}
