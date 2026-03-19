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

func TestComposeSMS(t *testing.T) {
	t.Parallel()

	type test struct {
		name                  string
		config                map[string]any
		templateSender        *management.SenderIdentity
		providerDefaultSender *management.SenderIdentity
		template              management.Template
		user                  *subjects.User
		wantFrom              string
		wantErr               bool
		errContains           string
	}

	senderIdentity := func(address string) *management.SenderIdentity {
		data, _ := json.Marshal(map[string]any{"address": address})
		return &management.SenderIdentity{Traits: data}
	}

	tests := []test{
		{
			name: "uses template sender when specified",
			config: map[string]any{
				"accountSid": "test-sid",
				"authToken":  "test-token",
			},
			templateSender:        senderIdentity("+11111111111"),
			providerDefaultSender: senderIdentity("+10000000000"),
			template: management.Template{
				Data: json.RawMessage(`{
					"body": "Test message"
				}`),
			},
			user:     &subjects.User{Phone: ptr("+12222222222")},
			wantFrom: "+11111111111",
			wantErr:  false,
		},
		{
			name: "uses provider default_from when template sender is nil",
			config: map[string]any{
				"accountSid": "test-sid",
				"authToken":  "test-token",
			},
			templateSender:        nil,
			providerDefaultSender: senderIdentity("+10000000000"),
			template: management.Template{
				Data: json.RawMessage(`{
					"body": "Test message"
				}`),
			},
			user:     &subjects.User{Phone: ptr("+12222222222")},
			wantFrom: "+10000000000",
			wantErr:  false,
		},
		{
			name: "template sender takes priority over provider default",
			config: map[string]any{
				"accountSid": "test-sid",
				"authToken":  "test-token",
			},
			templateSender:        senderIdentity("+11111111111"),
			providerDefaultSender: senderIdentity("+10000000000"),
			template: management.Template{
				Data: json.RawMessage(`{
					"body": "Test message"
				}`),
			},
			user:     &subjects.User{Phone: ptr("+12222222222")},
			wantFrom: "+11111111111",
			wantErr:  false,
		},
		{
			name: "errors when no from number available",
			config: map[string]any{
				"accountSid": "test-sid",
				"authToken":  "test-token",
			},
			templateSender:        nil,
			providerDefaultSender: nil,
			template: management.Template{
				Data: json.RawMessage(`{
					"body": "Test message"
				}`),
			},
			user:        &subjects.User{Phone: ptr("+12222222222")},
			wantErr:     true,
			errContains: "no from number specified",
		},
		{
			name: "errors when user has no phone",
			config: map[string]any{
				"accountSid": "test-sid",
				"authToken":  "test-token",
			},
			templateSender:        senderIdentity("+11111111111"),
			providerDefaultSender: senderIdentity("+10000000000"),
			template: management.Template{
				Data: json.RawMessage(`{
					"body": "Test message"
				}`),
			},
			user:        &subjects.User{Phone: nil},
			wantErr:     true,
			errContains: "user has no phone number",
		},
		{
			name: "uses provider default when template sender is nil",
			config: map[string]any{
				"accountSid": "test-sid",
				"authToken":  "test-token",
			},
			templateSender:        nil,
			providerDefaultSender: senderIdentity("+10000000000"),
			template: management.Template{
				Data: json.RawMessage(`{
					"body": "Test message"
				}`),
			},
			user:     &subjects.User{Phone: ptr("+12222222222")},
			wantFrom: "+10000000000",
			wantErr:  false,
		},
		{
			name:                  "handles nil senders with template sender",
			config:                map[string]any{},
			templateSender:        senderIdentity("+11111111111"),
			providerDefaultSender: nil,
			template: management.Template{
				Data: json.RawMessage(`{
					"body": "Test message"
				}`),
			},
			user:     &subjects.User{Phone: ptr("+12222222222")},
			wantFrom: "+11111111111",
			wantErr:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ComposeSMS(context.Background(), tc.templateSender, tc.providerDefaultSender, tc.config, tc.template, tc.user)

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
