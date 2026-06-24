package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/lunogram/platform/pkg/modules/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComposeSendEmailRequest(t *testing.T) {
	t.Run("uses configured domain and maps fields", func(t *testing.T) {
		cc := "cc@example.com"
		replyTo := "reply@example.com"
		email := providers.EmailPayload{
			To:      "recipient@example.com",
			From:    providers.EmailAddress{Name: "Sender", Address: "sender@example.com"},
			Subject: "Test subject",
			Text:    "Hello",
			HTML:    "<p>Hello</p>",
			Cc:      &cc,
			ReplyTo: &replyTo,
			Headers: map[string]string{"X-Custom": "value"},
		}

		req, err := ComposeSendEmailRequest(email, "mg.example.com")
		require.NoError(t, err)

		assert.Equal(t, "mg.example.com", req.Domain)
		assert.Equal(t, "Sender <sender@example.com>", req.Form.Get("from"))
		assert.Equal(t, "recipient@example.com", req.Form.Get("to"))
		assert.Equal(t, "Test subject", req.Form.Get("subject"))
		assert.Equal(t, "Hello", req.Form.Get("text"))
		assert.Equal(t, "<p>Hello</p>", req.Form.Get("html"))
		assert.Equal(t, "cc@example.com", req.Form.Get("cc"))
		assert.Equal(t, "reply@example.com", req.Form.Get("h:Reply-To"))
		assert.Equal(t, "value", req.Form.Get("h:X-Custom"))
	})

	t.Run("falls back to the from address domain", func(t *testing.T) {
		email := providers.EmailPayload{
			To:      "recipient@example.com",
			From:    providers.EmailAddress{Address: "sender@mg.example.com"},
			Subject: "Test",
			Text:    "Hello",
		}

		req, err := ComposeSendEmailRequest(email, "")
		require.NoError(t, err)
		assert.Equal(t, "mg.example.com", req.Domain)
	})

	t.Run("requires text or html content", func(t *testing.T) {
		email := providers.EmailPayload{
			To:      "recipient@example.com",
			From:    providers.EmailAddress{Address: "sender@mg.example.com"},
			Subject: "Test",
		}

		_, err := ComposeSendEmailRequest(email, "mg.example.com")
		assert.Error(t, err)
	})
}

func TestResolveMailgunAPIBase(t *testing.T) {
	cases := map[string]string{
		"":   "https://api.mailgun.net",
		"US": "https://api.mailgun.net",
		"us": "https://api.mailgun.net",
		"EU": "https://api.eu.mailgun.net",
		"eu": "https://api.eu.mailgun.net",
	}
	for region, want := range cases {
		got, err := resolveMailgunAPIBase(region)
		require.NoError(t, err, "region %q", region)
		assert.Equal(t, want, got, "region %q", region)
	}

	_, err := resolveMailgunAPIBase("APAC")
	assert.Error(t, err)
}

func TestValidateConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		errs := validateConfig(Config{APIKey: "key", Domain: "mg.example.com", APIRegion: "EU"})
		assert.Empty(t, errs)
	})

	t.Run("missing fields and bad region", func(t *testing.T) {
		errs := validateConfig(Config{APIRegion: "APAC"})
		assert.Contains(t, errs, "apiKey")
		assert.Contains(t, errs, "domain")
		assert.Contains(t, errs, "apiRegion")
	})
}

func TestClassifyHTTPStatus(t *testing.T) {
	assert.Equal(t, ExitTransient, classifyHTTPStatus(429))
	assert.Equal(t, ExitTransient, classifyHTTPStatus(503))
	assert.Equal(t, ExitPermanent, classifyHTTPStatus(400))
	assert.Equal(t, ExitPermanent, classifyHTTPStatus(401))
}

func TestParseMailgunWebhookEvent(t *testing.T) {
	t.Run("maps delivered event", func(t *testing.T) {
		body := buildWebhookBody(t, "delivered", nil)
		event, ok, err := parseMailgunWebhookEvent(body)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, providers.EventDelivered, event.EventName)
		assert.Equal(t, "msg-1", event.MessageID)
	})

	t.Run("temporary failure maps to deferred", func(t *testing.T) {
		body := buildWebhookBody(t, "failed", map[string]any{
			"delivery-status": map[string]any{"severity": "temporary"},
		})
		event, ok, err := parseMailgunWebhookEvent(body)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, providers.EventDeferred, event.EventName)
	})

	t.Run("permanent failure maps to bounced", func(t *testing.T) {
		body := buildWebhookBody(t, "failed", map[string]any{
			"delivery-status": map[string]any{"severity": "permanent"},
		})
		event, ok, err := parseMailgunWebhookEvent(body)
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, providers.EventBounced, event.EventName)
	})

	t.Run("unknown event is skipped", func(t *testing.T) {
		body := buildWebhookBody(t, "something-else", nil)
		_, ok, err := parseMailgunWebhookEvent(body)
		require.NoError(t, err)
		assert.False(t, ok)
	})
}

func TestVerifyMailgunWebhookSignature(t *testing.T) {
	signingKey := "signing-key"
	timestamp := fmt.Sprintf("%d", time.Now().UTC().Unix())
	token := "token-123"

	mac := hmac.New(sha256.New, []byte(signingKey))
	_, _ = mac.Write([]byte(timestamp + token))
	signature := hex.EncodeToString(mac.Sum(nil))

	body := buildWebhookBodyWithSignature(t, timestamp, token, signature)
	assert.NoError(t, verifyMailgunWebhookSignature(signingKey, body))

	tampered := buildWebhookBodyWithSignature(t, timestamp, token, "deadbeef")
	assert.Error(t, verifyMailgunWebhookSignature(signingKey, tampered))
}

func buildWebhookBody(t *testing.T, eventType string, extra map[string]any) []byte {
	t.Helper()
	eventData := map[string]any{
		"event":     eventType,
		"id":        "msg-1",
		"timestamp": float64(time.Now().UTC().Unix()),
	}
	for k, v := range extra {
		eventData[k] = v
	}
	raw, err := json.Marshal(eventData)
	require.NoError(t, err)
	payload, err := json.Marshal(map[string]any{"event-data": json.RawMessage(raw)})
	require.NoError(t, err)
	return payload
}

func buildWebhookBodyWithSignature(t *testing.T, timestamp, token, signature string) []byte {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"signature": map[string]string{
			"timestamp": timestamp,
			"token":     token,
			"signature": signature,
		},
		"event-data": map[string]any{"event": "delivered", "id": "msg-1"},
	})
	require.NoError(t, err)
	return payload
}
