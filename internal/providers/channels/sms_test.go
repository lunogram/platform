package channels

import (
	"encoding/json"
	"testing"

	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComposeSMS(t *testing.T) {
	t.Parallel()

	type test struct {
		name        string
		config      map[string]any
		template    management.Template
		user        *users.User
		wantFrom    string
		wantErr     bool
		errContains string
	}

	tests := []test{
		{
			name: "uses template from when provider unlocked and template specifies",
			config: map[string]any{
				"data": map[string]any{
					"accountSid":                 "test-sid",
					"authToken":                  "test-token",
					ProviderKeyDefaultFrom:       "+10000000000",
					ProviderKeyDefaultFromLocked: false,
				},
			},
			template: management.Template{
				Data: json.RawMessage(`{
					"from": "+11111111111",
					"body": "Test message"
				}`),
			},
			user:     &users.User{Phone: ptr("+12222222222")},
			wantFrom: "+11111111111",
			wantErr:  false,
		},
		{
			name: "uses provider default_from when template empty",
			config: map[string]any{
				"data": map[string]any{
					"accountSid":                 "test-sid",
					"authToken":                  "test-token",
					ProviderKeyDefaultFrom:       "+10000000000",
					ProviderKeyDefaultFromLocked: false,
				},
			},
			template: management.Template{
				Data: json.RawMessage(`{
					"from": "",
					"body": "Test message"
				}`),
			},
			user:     &users.User{Phone: ptr("+12222222222")},
			wantFrom: "+10000000000",
			wantErr:  false,
		},
		{
			name: "uses provider default_from when locked (ignores template)",
			config: map[string]any{
				"data": map[string]any{
					"accountSid":                 "test-sid",
					"authToken":                  "test-token",
					ProviderKeyDefaultFrom:       "+10000000000",
					ProviderKeyDefaultFromLocked: true,
				},
			},
			template: management.Template{
				Data: json.RawMessage(`{
					"from": "+11111111111",
					"body": "Test message"
				}`),
			},
			user:     &users.User{Phone: ptr("+12222222222")},
			wantFrom: "+10000000000",
			wantErr:  false,
		},
		{
			name: "errors when no from number available",
			config: map[string]any{
				"data": map[string]any{
					"accountSid": "test-sid",
					"authToken":  "test-token",
				},
			},
			template: management.Template{
				Data: json.RawMessage(`{
					"body": "Test message"
				}`),
			},
			user:        &users.User{Phone: ptr("+12222222222")},
			wantErr:     true,
			errContains: "no from number specified",
		},
		{
			name: "errors when user has no phone",
			config: map[string]any{
				"data": map[string]any{
					"accountSid":           "test-sid",
					"authToken":            "test-token",
					ProviderKeyDefaultFrom: "+10000000000",
				},
			},
			template: management.Template{
				Data: json.RawMessage(`{
					"from": "+11111111111",
					"body": "Test message"
				}`),
			},
			user:        &users.User{Phone: nil},
			wantErr:     true,
			errContains: "user has no phone number",
		},
		{
			name: "uses provider default when template has no from field at all",
			config: map[string]any{
				"data": map[string]any{
					"accountSid":           "test-sid",
					"authToken":            "test-token",
					ProviderKeyDefaultFrom: "+10000000000",
				},
			},
			template: management.Template{
				Data: json.RawMessage(`{
					"body": "Test message"
				}`),
			},
			user:     &users.User{Phone: ptr("+12222222222")},
			wantFrom: "+10000000000",
			wantErr:  false,
		},
		{
			name:   "handles nil data in config",
			config: map[string]any{},
			template: management.Template{
				Data: json.RawMessage(`{
					"from": "+11111111111",
					"body": "Test message"
				}`),
			},
			user:     &users.User{Phone: ptr("+12222222222")},
			wantFrom: "+11111111111",
			wantErr:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ComposeSMS(tc.config, tc.template, tc.user)

			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					assert.Contains(t, err.Error(), tc.errContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			// Get the SMS payload from the request
			payload, err := result.GetSMSPayload()
			require.NoError(t, err)

			assert.Equal(t, tc.wantFrom, payload.From)
			assert.Equal(t, *tc.user.Phone, payload.To)
		})
	}
}
