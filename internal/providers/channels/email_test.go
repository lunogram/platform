package channels

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T {
	return &v
}

func TestComposeEmail(t *testing.T) {
	t.Parallel()

	type test struct {
		name                  string
		config                map[string]any
		template              management.Template
		templateSender        *management.SenderIdentity
		providerDefaultSender *management.SenderIdentity
		user                  *subjects.User
		wantFrom              string
		wantName              string
		wantErr               bool
		errContains           string
	}

	senderIdentity := func(address string, extras ...any) *management.SenderIdentity {
		traits := map[string]any{"address": address}
		for i := 0; i+1 < len(extras); i += 2 {
			traits[extras[i].(string)] = extras[i+1]
		}
		data, _ := json.Marshal(traits)
		return &management.SenderIdentity{Traits: data}
	}

	tests := []test{
		{
			name: "uses template sender when template specifies address and name",
			config: map[string]any{
				"apiKey": "test-api-key",
			},
			templateSender:        senderIdentity("custom@example.com", "name", "Custom Trait Name"),
			providerDefaultSender: senderIdentity("default@example.com"),
			template: management.Template{
				Data: json.RawMessage(`{
					"from": {"name": "Custom Name"},
					"subject": "Test Subject",
					"html": "<p>Test</p>"
				}`),
			},
			user:     &subjects.User{Email: ptr("user@example.com")},
			wantFrom: "custom@example.com",
			wantName: "Custom Name",
			wantErr:  false,
		},
		{
			name: "uses provider default sender name when template sender is nil",
			config: map[string]any{
				"apiKey": "test-api-key",
			},
			templateSender:        nil,
			providerDefaultSender: senderIdentity("default@example.com", "name", "Default Name"),
			template: management.Template{
				Data: json.RawMessage(`{
					"from": {"name": ""},
					"subject": "Test Subject",
					"html": "<p>Test</p>"
				}`),
			},
			user:     &subjects.User{Email: ptr("user@example.com")},
			wantFrom: "default@example.com",
			wantName: "Default Name",
			wantErr:  false,
		},
		{
			name: "errors when no from address available",
			config: map[string]any{
				"apiKey": "test-api-key",
			},
			templateSender:        nil,
			providerDefaultSender: nil,
			template: management.Template{
				Data: json.RawMessage(`{
					"from": {"name": ""},
					"subject": "Test Subject",
					"html": "<p>Test</p>"
				}`),
			},
			user:        &subjects.User{Email: ptr("user@example.com")},
			wantErr:     true,
			errContains: "no from address specified",
		},
		{
			name: "errors when user has no email",
			config: map[string]any{
				"apiKey": "test-api-key",
			},
			templateSender:        senderIdentity("sender@example.com", "name", "Sender"),
			providerDefaultSender: senderIdentity("default@example.com"),
			template: management.Template{
				Data: json.RawMessage(`{
					"from": {"name": "Sender"},
					"subject": "Test Subject",
					"html": "<p>Test</p>"
				}`),
			},
			user:        &subjects.User{Email: nil},
			wantErr:     true,
			errContains: "user has no email address",
		},
		{
			name: "uses template address with no name when no name trait exists",
			config: map[string]any{
				"apiKey": "test-api-key",
			},
			templateSender:        senderIdentity("custom@example.com"),
			providerDefaultSender: senderIdentity("default@example.com"),
			template: management.Template{
				Data: json.RawMessage(`{
					"subject": "Test Subject",
					"html": "<p>Test</p>"
				}`),
			},
			user:     &subjects.User{Email: ptr("user@example.com")},
			wantFrom: "custom@example.com",
			wantName: "",
			wantErr:  false,
		},
		{
			name: "uses provider default sender trait name when template name is empty",
			config: map[string]any{
				"apiKey": "test-api-key",
			},
			templateSender:        senderIdentity("custom@example.com"),
			providerDefaultSender: senderIdentity("default@example.com", "name", "Fallback Name"),
			template: management.Template{
				Data: json.RawMessage(`{
					"from": {"name": ""},
					"subject": "Test Subject",
					"html": "<p>Test</p>"
				}`),
			},
			user:     &subjects.User{Email: ptr("user@example.com")},
			wantFrom: "custom@example.com",
			wantName: "Fallback Name",
			wantErr:  false,
		},
		{
			name:                  "handles nil data in config",
			config:                map[string]any{},
			templateSender:        senderIdentity("sender@example.com", "name", "Sender Trait"),
			providerDefaultSender: nil,
			template: management.Template{
				Data: json.RawMessage(`{
					"from": {"name": "Sender"},
					"subject": "Test Subject",
					"html": "<p>Test</p>"
				}`),
			},
			user:     &subjects.User{Email: ptr("user@example.com")},
			wantFrom: "sender@example.com",
			wantName: "Sender",
			wantErr:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ComposeEmail(context.Background(), tc.templateSender, tc.providerDefaultSender, tc.config, tc.template, tc.user)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			// Get the email payload from the request
			payload, err := result.GetEmailPayload()
			require.NoError(t, err)

			assert.Equal(t, tc.wantFrom, payload.From.Address)
			assert.Equal(t, tc.wantName, payload.From.Name)
			assert.Equal(t, *tc.user.Email, payload.To)
		})
	}
}
