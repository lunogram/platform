package channels

import (
	"encoding/json"
	"fmt"

	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/lunogram/platform/pkg/modules/providers"
)

// WebhookTemplateData represents webhook template content.
type WebhookTemplateData struct {
	Method   string            `json:"method"`
	Endpoint string            `json:"endpoint"`
	Body     map[string]any    `json:"body,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
	CacheKey string            `json:"cache_key,omitempty"`
}

func ComposeWebhook(config map[string]any, template management.Template, user *subjects.User) (*providers.SendRequest[map[string]any], error) {
	var data WebhookTemplateData
	if err := json.Unmarshal(template.Data, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal webhook template data: %w", err)
	}

	if data.Endpoint == "" {
		return nil, fmt.Errorf("webhook endpoint is required")
	}

	if data.Method == "" {
		data.Method = "POST"
	}

	payload := providers.WebhookPayload{
		Method:   data.Method,
		Endpoint: data.Endpoint,
		Body:     data.Body,
		Headers:  data.Headers,
		CacheKey: data.CacheKey,
	}

	return providers.NewWebhookRequest(config, payload)
}
