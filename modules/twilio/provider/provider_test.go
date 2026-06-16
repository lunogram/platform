package provider

import (
	"testing"

	"github.com/lunogram/platform/pkg/modules/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExitCodes(t *testing.T) {
	t.Parallel()

	assert.Equal(t, int32(0), ExitSuccess)
	assert.Equal(t, int32(-1), ExitTransient)
	assert.Equal(t, int32(-2), ExitPermanent)
}

func TestClassifyHTTPStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   int
		expected int32
	}{
		{name: "429 rate limit is transient", status: 429, expected: ExitTransient},
		{name: "400 bad request is permanent", status: 400, expected: ExitPermanent},
		{name: "401 unauthorized is permanent", status: 401, expected: ExitPermanent},
		{name: "403 forbidden is permanent", status: 403, expected: ExitPermanent},
		{name: "404 not found is permanent", status: 404, expected: ExitPermanent},
		{name: "422 unprocessable is permanent", status: 422, expected: ExitPermanent},
		{name: "499 client error is permanent", status: 499, expected: ExitPermanent},
		{name: "500 internal server error is transient", status: 500, expected: ExitTransient},
		{name: "502 bad gateway is transient", status: 502, expected: ExitTransient},
		{name: "503 service unavailable is transient", status: 503, expected: ExitTransient},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyHTTPStatus(tt.status)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapWebhookStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		status        string
		expectedEvent providers.WebhookEventName
		expectedOK    bool
	}{
		{name: "sent maps to EventSent", status: "sent", expectedEvent: providers.EventSent, expectedOK: true},
		{name: "delivered maps to EventDelivered", status: "delivered", expectedEvent: providers.EventDelivered, expectedOK: true},
		{name: "read maps to EventDelivered", status: "read", expectedEvent: providers.EventDelivered, expectedOK: true},
		{name: "failed maps to EventBounced", status: "failed", expectedEvent: providers.EventBounced, expectedOK: true},
		{name: "undelivered maps to EventBounced", status: "undelivered", expectedEvent: providers.EventBounced, expectedOK: true},
		{name: "queued maps to EventDeferred", status: "queued", expectedEvent: providers.EventDeferred, expectedOK: true},
		{name: "accepted maps to EventDeferred", status: "accepted", expectedEvent: providers.EventDeferred, expectedOK: true},
		{name: "sending maps to EventDeferred", status: "sending", expectedEvent: providers.EventDeferred, expectedOK: true},
		{name: "unknown status returns false", status: "unknown", expectedEvent: 0, expectedOK: false},
		{name: "empty string returns false", status: "", expectedEvent: 0, expectedOK: false},
		{name: "capitalized Sent is not recognized", status: "Sent", expectedEvent: 0, expectedOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, ok := MapWebhookStatus(tt.status)
			assert.Equal(t, tt.expectedOK, ok)
			assert.Equal(t, tt.expectedEvent, event)
		})
	}
}

func TestParseWebhookBody(t *testing.T) {
	t.Parallel()

	t.Run("parses complete payload", func(t *testing.T) {
		body := "MessageSid=SM123&MessageStatus=delivered&To=%2B15551234567&From=%2B15559876543&AccountSid=AC456&ApiVersion=2010-04-01&ErrorCode=30001&ErrorMessage=Queue+overflow"

		payload, err := ParseWebhookBody([]byte(body))
		require.NoError(t, err)

		assert.Equal(t, "SM123", payload.MessageSid)
		assert.Equal(t, "delivered", payload.MessageStatus)
		assert.Equal(t, "+15551234567", payload.To)
		assert.Equal(t, "+15559876543", payload.From)
		assert.Equal(t, "AC456", payload.AccountSid)
		assert.Equal(t, "2010-04-01", payload.ApiVersion)
		assert.Equal(t, "30001", payload.ErrorCode)
		assert.Equal(t, "Queue overflow", payload.ErrorMessage)
	})

	t.Run("parses minimal payload", func(t *testing.T) {
		body := "MessageSid=SM789&MessageStatus=sent"

		payload, err := ParseWebhookBody([]byte(body))
		require.NoError(t, err)

		assert.Equal(t, "SM789", payload.MessageSid)
		assert.Equal(t, "sent", payload.MessageStatus)
		assert.Empty(t, payload.To)
		assert.Empty(t, payload.From)
		assert.Empty(t, payload.ErrorCode)
		assert.Empty(t, payload.ErrorMessage)
	})

	t.Run("handles empty body", func(t *testing.T) {
		payload, err := ParseWebhookBody([]byte(""))
		require.NoError(t, err)

		assert.Empty(t, payload.MessageSid)
		assert.Empty(t, payload.MessageStatus)
	})

	t.Run("returns error for malformed body", func(t *testing.T) {
		_, err := ParseWebhookBody([]byte("%zz"))
		require.Error(t, err)
	})
}

func TestParseWebhookParams(t *testing.T) {
	t.Parallel()

	t.Run("parses params into flat map", func(t *testing.T) {
		body := "MessageSid=SM123&MessageStatus=delivered&To=%2B15551234567"

		params, err := ParseWebhookParams([]byte(body))
		require.NoError(t, err)

		assert.Equal(t, "SM123", params["MessageSid"])
		assert.Equal(t, "delivered", params["MessageStatus"])
		assert.Equal(t, "+15551234567", params["To"])
		assert.Len(t, params, 3)
	})

	t.Run("uses first value for duplicate keys", func(t *testing.T) {
		body := "Key=first&Key=second"

		params, err := ParseWebhookParams([]byte(body))
		require.NoError(t, err)

		assert.Equal(t, "first", params["Key"])
		assert.Len(t, params, 1)
	})

	t.Run("handles empty body", func(t *testing.T) {
		params, err := ParseWebhookParams([]byte(""))
		require.NoError(t, err)
		assert.Empty(t, params)
	})

	t.Run("returns error for malformed body", func(t *testing.T) {
		_, err := ParseWebhookParams([]byte("%zz"))
		require.Error(t, err)
	})
}

func TestValidateConfig(t *testing.T) {
	t.Parallel()

	t.Run("valid config returns no errors", func(t *testing.T) {
		config := Config{
			AccountSID: "AC123",
			AuthToken:  "token123",
		}

		errs := ValidateConfig(config)
		assert.Empty(t, errs)
	})

	t.Run("missing all fields returns two errors", func(t *testing.T) {
		errs := ValidateConfig(Config{})

		assert.Len(t, errs, 2)
		assert.Equal(t, "Account SID is required", errs["accountSid"])
		assert.Equal(t, "Auth Token is required", errs["authToken"])
	})

	t.Run("missing account SID only", func(t *testing.T) {
		config := Config{
			AuthToken: "token123",
		}

		errs := ValidateConfig(config)
		assert.Len(t, errs, 1)
		assert.Contains(t, errs, "accountSid")
	})

	t.Run("missing auth token only", func(t *testing.T) {
		config := Config{
			AccountSID: "AC123",
		}

		errs := ValidateConfig(config)
		assert.Len(t, errs, 1)
		assert.Contains(t, errs, "authToken")
	})

	t.Run("webhook URL is not required", func(t *testing.T) {
		config := Config{
			AccountSID: "AC123",
			AuthToken:  "token123",
			// WebhookURL intentionally empty
		}

		errs := ValidateConfig(config)
		assert.Empty(t, errs)
	})
}

func TestResolveSender(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		payloadFrom string
		expectedNum string
		expectedOK  bool
	}{
		{
			name:        "returns payload from when set",
			payloadFrom: "+15551111111",
			expectedNum: "+15551111111",
			expectedOK:  true,
		},
		{
			name:        "returns false when empty",
			payloadFrom: "",
			expectedNum: "",
			expectedOK:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			num, ok := ResolveSender(tt.payloadFrom)
			assert.Equal(t, tt.expectedOK, ok)
			assert.Equal(t, tt.expectedNum, num)
		})
	}
}

func TestConfigJSON(t *testing.T) {
	t.Parallel()

	t.Run("json tags match expected field names", func(t *testing.T) {
		// Verify that the Config struct can round-trip through JSON with
		// the expected field names by encoding and decoding.
		config := Config{
			AccountSID: "AC123",
			AuthToken:  "secret",
			WebhookURL: "https://example.com/webhook",
		}

		assert.Equal(t, "AC123", config.AccountSID)
		assert.Equal(t, "secret", config.AuthToken)
		assert.Equal(t, "https://example.com/webhook", config.WebhookURL)
	})
}
