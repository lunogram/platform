package providers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Providers sign the exact bytes they send, so the body must reach the module
// exactly as the host received it.
func TestWebhookRequestBodyRoundTrip(t *testing.T) {
	t.Parallel()

	bodies := map[string]string{
		"json with spacing and html characters": "{\n  \"subject\": \"Terms & <conditions>\"\n}",
		"form encoded":                          "MessageSid=SM123&MessageStatus=delivered&To=%2B15551234567",
	}

	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			encoded, err := json.Marshal(WebhookRequest{Body: []byte(body)})
			require.NoError(t, err)

			var decoded WebhookRequest
			require.NoError(t, json.Unmarshal(encoded, &decoded))
			assert.Equal(t, body, string(decoded.Body))
		})
	}
}
