package channels

import (
	"encoding/json"
	"testing"

	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T {
	return &v
}

func TestComposeEmail(t *testing.T) {
	t.Parallel()

	type test struct {
		name        string
		config      map[string]any
		template    management.Template
		user        *users.User
		wantFrom    string
		wantName    string
		wantErr     bool
		errContains string
	}

	tests := []test{
		{
			name: "uses template from when provider unlocked and template specifies",
			config: map[string]any{
				"data": map[string]any{
					"apiKey":              "test-api-key",
					"default_from":        "default@example.com",
					"default_from_name":   "Default Name",
					"default_from_locked": false,
				},
			},
			template: management.Template{
				Data: json.RawMessage(`{
					"from": {"email": "custom@example.com", "name": "Custom Name"},
					"subject": "Test Subject",
					"html": "<p>Test</p>"
				}`),
			},
			user:     &users.User{Email: ptr("user@example.com")},
			wantFrom: "custom@example.com",
			wantName: "Custom Name",
			wantErr:  false,
		},
		{
			name: "uses provider default_from when template empty",
			config: map[string]any{
				"data": map[string]any{
					"apiKey":              "test-api-key",
					"default_from":        "default@example.com",
					"default_from_name":   "Default Name",
					"default_from_locked": false,
				},
			},
			template: management.Template{
				Data: json.RawMessage(`{
					"from": {"email": "", "name": ""},
					"subject": "Test Subject",
					"html": "<p>Test</p>"
				}`),
			},
			user:     &users.User{Email: ptr("user@example.com")},
			wantFrom: "default@example.com",
			wantName: "Default Name",
			wantErr:  false,
		},
		{
			name: "uses provider default_from when locked (ignores template)",
			config: map[string]any{
				"data": map[string]any{
					"apiKey":              "test-api-key",
					"default_from":        "locked@example.com",
					"default_from_name":   "Locked Name",
					"default_from_locked": true,
				},
			},
			template: management.Template{
				Data: json.RawMessage(`{
					"from": {"email": "custom@example.com", "name": "Custom Name"},
					"subject": "Test Subject",
					"html": "<p>Test</p>"
				}`),
			},
			user:     &users.User{Email: ptr("user@example.com")},
			wantFrom: "locked@example.com",
			wantName: "Locked Name",
			wantErr:  false,
		},
		{
			name: "errors when no from address available",
			config: map[string]any{
				"data": map[string]any{
					"apiKey": "test-api-key",
				},
			},
			template: management.Template{
				Data: json.RawMessage(`{
					"from": {"email": "", "name": ""},
					"subject": "Test Subject",
					"html": "<p>Test</p>"
				}`),
			},
			user:        &users.User{Email: ptr("user@example.com")},
			wantErr:     true,
			errContains: "no from address specified",
		},
		{
			name: "errors when user has no email",
			config: map[string]any{
				"data": map[string]any{
					"apiKey":       "test-api-key",
					"default_from": "default@example.com",
				},
			},
			template: management.Template{
				Data: json.RawMessage(`{
					"from": {"email": "sender@example.com", "name": "Sender"},
					"subject": "Test Subject",
					"html": "<p>Test</p>"
				}`),
			},
			user:        &users.User{Email: nil},
			wantErr:     true,
			errContains: "user has no email address",
		},
		{
			name: "uses template from with no name when provider has no default_from_name",
			config: map[string]any{
				"data": map[string]any{
					"apiKey":       "test-api-key",
					"default_from": "default@example.com",
				},
			},
			template: management.Template{
				Data: json.RawMessage(`{
					"from": {"email": "custom@example.com"},
					"subject": "Test Subject",
					"html": "<p>Test</p>"
				}`),
			},
			user:     &users.User{Email: ptr("user@example.com")},
			wantFrom: "custom@example.com",
			wantName: "",
			wantErr:  false,
		},
		{
			name: "uses provider default_from_name fallback when template name is empty",
			config: map[string]any{
				"data": map[string]any{
					"apiKey":            "test-api-key",
					"default_from_name": "Fallback Name",
				},
			},
			template: management.Template{
				Data: json.RawMessage(`{
					"from": {"email": "custom@example.com", "name": ""},
					"subject": "Test Subject",
					"html": "<p>Test</p>"
				}`),
			},
			user:     &users.User{Email: ptr("user@example.com")},
			wantFrom: "custom@example.com",
			wantName: "Fallback Name",
			wantErr:  false,
		},
		{
			name:   "handles nil data in config",
			config: map[string]any{},
			template: management.Template{
				Data: json.RawMessage(`{
					"from": {"email": "sender@example.com", "name": "Sender"},
					"subject": "Test Subject",
					"html": "<p>Test</p>"
				}`),
			},
			user:     &users.User{Email: ptr("user@example.com")},
			wantFrom: "sender@example.com",
			wantName: "Sender",
			wantErr:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ComposeEmail(tc.config, tc.template, tc.user)

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
