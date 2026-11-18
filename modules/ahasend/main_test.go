package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/lunogram/platform/pkg/modules/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComposeSendEmailRequest(t *testing.T) {
	t.Run("maps fields onto a conversation message", func(t *testing.T) {
		cc := "cc@example.com"
		bcc := "bcc@example.com"
		replyTo := "reply@example.com"
		email := providers.EmailPayload{
			To:      "recipient@example.com",
			From:    providers.EmailAddress{Name: "Sender", Address: "sender@example.com"},
			Subject: "Test subject",
			Text:    "Hello",
			HTML:    "<p>Hello</p>",
			Cc:      &cc,
			Bcc:     &bcc,
			ReplyTo: &replyTo,
			Headers: map[string]string{"X-Custom": "value"},
			List:    &providers.ListUnsubscribe{Unsubscribe: "<https://example.com/unsubscribe>"},
		}

		req, err := ComposeSendEmailRequest(email)
		require.NoError(t, err)

		body, err := json.Marshal(req)
		require.NoError(t, err)
		assert.JSONEq(t, `{
			"from": {"email": "sender@example.com", "name": "Sender"},
			"to": [{"email": "recipient@example.com"}],
			"cc": [{"email": "cc@example.com"}],
			"bcc": [{"email": "bcc@example.com"}],
			"reply_to": {"email": "reply@example.com"},
			"subject": "Test subject",
			"text_content": "Hello",
			"html_content": "<p>Hello</p>",
			"headers": {"X-Custom": "value", "List-Unsubscribe": "<https://example.com/unsubscribe>"}
		}`, string(body))
	})

	t.Run("omits optional fields", func(t *testing.T) {
		email := providers.EmailPayload{
			To:      "recipient@example.com",
			From:    providers.EmailAddress{Address: "sender@example.com"},
			Subject: "Test",
			HTML:    "<p>Hello</p>",
		}

		req, err := ComposeSendEmailRequest(email)
		require.NoError(t, err)

		body, err := json.Marshal(req)
		require.NoError(t, err)
		assert.JSONEq(t, `{
			"from": {"email": "sender@example.com"},
			"to": [{"email": "recipient@example.com"}],
			"subject": "Test",
			"html_content": "<p>Hello</p>"
		}`, string(body))
	})

	t.Run("requires text or html content", func(t *testing.T) {
		email := providers.EmailPayload{
			To:      "recipient@example.com",
			From:    providers.EmailAddress{Address: "sender@example.com"},
			Subject: "Test",
		}

		_, err := ComposeSendEmailRequest(email)
		assert.Error(t, err)
	})
}

func TestMessageID(t *testing.T) {
	t.Run("returns the copy sent to the To recipient", func(t *testing.T) {
		var resp ahaSendMessageResponse
		require.NoError(t, json.Unmarshal([]byte(`{"object":"list","data":[
			{"object":"message","id":"cc-copy","recipient":{"email":"cc@example.com","name":""},"status":"queued","error":null},
			{"object":"message","id":"to-copy","recipient":{"email":"Recipient@Example.com","name":""},"status":"queued","error":null}
		]}`), &resp))

		id, err := messageID(resp, "recipient@example.com")
		require.NoError(t, err)
		assert.Equal(t, "to-copy", id)
	})

	t.Run("reports a recipient AhaSend did not send to", func(t *testing.T) {
		var resp ahaSendMessageResponse
		require.NoError(t, json.Unmarshal([]byte(`{"object":"list","data":[
			{"object":"message","id":null,"recipient":{"email":"recipient@example.com","name":""},"status":"error","error":"recipient is suppressed"}
		]}`), &resp))

		_, err := messageID(resp, "recipient@example.com")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "recipient is suppressed")
	})

	t.Run("rejects an empty response", func(t *testing.T) {
		_, err := messageID(ahaSendMessageResponse{}, "recipient@example.com")
		assert.Error(t, err)
	})
}

func TestValidateConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		errs := validateConfig(Config{APIKey: "aha-sk-key", AccountID: "0199f1c2-7d3a-7b4e-9a1f-2c3d4e5f6a7b"})
		assert.Empty(t, errs)
	})

	t.Run("missing fields", func(t *testing.T) {
		errs := validateConfig(Config{})
		assert.Contains(t, errs, "apiKey")
		assert.Contains(t, errs, "accountId")
	})

	t.Run("account ID must be a UUID", func(t *testing.T) {
		errs := validateConfig(Config{APIKey: "aha-sk-key", AccountID: "my-account"})
		assert.Contains(t, errs, "accountId")
	})
}

func TestClassifyHTTPStatus(t *testing.T) {
	assert.Equal(t, ExitTransient, classifyHTTPStatus(409))
	assert.Equal(t, ExitTransient, classifyHTTPStatus(429))
	assert.Equal(t, ExitTransient, classifyHTTPStatus(503))
	assert.Equal(t, ExitPermanent, classifyHTTPStatus(400))
	assert.Equal(t, ExitPermanent, classifyHTTPStatus(403))
	assert.Equal(t, ExitPermanent, classifyHTTPStatus(422))
}

func TestParseAhaSendWebhookEvent(t *testing.T) {
	cases := map[string]providers.WebhookEventName{
		"message.reception":       providers.EventSent,
		"message.delivered":       providers.EventDelivered,
		"message.transient_error": providers.EventDeferred,
		"message.bounced":         providers.EventBounced,
		"message.failed":          providers.EventBounced,
		"message.suppressed":      providers.EventDropped,
		"message.opened":          providers.EventOpened,
		"message.clicked":         providers.EventClicked,
		"message.complained":      providers.EventComplained,
		"message.unsubscribed":    providers.EventUnsubscribed,
	}

	for eventType, want := range cases {
		t.Run(eventType, func(t *testing.T) {
			event, ok, err := parseAhaSendWebhookEvent(buildWebhookBody(t, eventType))
			require.NoError(t, err)
			require.True(t, ok)
			assert.Equal(t, want, event.EventName)
			assert.Equal(t, "msg-1", event.MessageID)
			assert.Equal(t, "2024-05-06T09:49:16.687031577Z", event.Timestamp)
			assert.Equal(t, "recipient@example.com", event.Data["recipient"])
		})
	}

	t.Run("non-message event is skipped", func(t *testing.T) {
		_, ok, err := parseAhaSendWebhookEvent(buildWebhookBody(t, "suppression.created"))
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("malformed body is an error", func(t *testing.T) {
		_, _, err := parseAhaSendWebhookEvent([]byte("not json"))
		assert.Error(t, err)
	})
}

func TestVerifyAhaSendWebhookSignature(t *testing.T) {
	secret := "aha-whsec-secret"
	body := buildWebhookBody(t, "message.delivered")
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	t.Run("accepts any matching v1 signature", func(t *testing.T) {
		headers := map[string]string{
			"webhook-id":        "delivery-1",
			"webhook-timestamp": timestamp,
			"webhook-signature": "v1,b3RoZXI= " + sign(secret, "delivery-1", timestamp, body),
		}
		assert.NoError(t, verifyAhaSendWebhookSignature(secret, headers, body))
	})

	t.Run("rejects a different secret", func(t *testing.T) {
		headers := map[string]string{
			"webhook-id":        "delivery-1",
			"webhook-timestamp": timestamp,
			"webhook-signature": sign("aha-whsec-other", "delivery-1", timestamp, body),
		}
		assert.Error(t, verifyAhaSendWebhookSignature(secret, headers, body))
	})

	t.Run("rejects a tampered body", func(t *testing.T) {
		headers := map[string]string{
			"webhook-id":        "delivery-1",
			"webhook-timestamp": timestamp,
			"webhook-signature": sign(secret, "delivery-1", timestamp, body),
		}
		assert.Error(t, verifyAhaSendWebhookSignature(secret, headers, buildWebhookBody(t, "message.bounced")))
	})

	t.Run("rejects a stale timestamp", func(t *testing.T) {
		stale := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
		headers := map[string]string{
			"webhook-id":        "delivery-1",
			"webhook-timestamp": stale,
			"webhook-signature": sign(secret, "delivery-1", stale, body),
		}
		assert.Error(t, verifyAhaSendWebhookSignature(secret, headers, body))
	})

	t.Run("rejects missing headers", func(t *testing.T) {
		assert.Error(t, verifyAhaSendWebhookSignature(secret, map[string]string{}, body))
	})
}

// sign builds a webhook-signature value the way AhaSend documents it:
// base64(HMAC-SHA256(secret bytes, id.timestamp.body)).
func sign(secret, id, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(id + "." + timestamp + "." + string(body)))
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func buildWebhookBody(t *testing.T, eventType string) []byte {
	t.Helper()

	body, err := json.Marshal(map[string]any{
		"type":       eventType,
		"webhook_id": "abe11757-2886-4b55-96f1-0e0afc95795a",
		"timestamp":  "2024-05-06T09:49:16.687031577Z",
		"data": map[string]any{
			"account_id":        "0199f1c2-7d3a-7b4e-9a1f-2c3d4e5f6a7b",
			"event":             "on_delivered",
			"from":              "sender@example.com",
			"recipient":         "recipient@example.com",
			"subject":           "Hello",
			"message_id_header": "<msg-1@example.com>",
			"id":                "msg-1",
		},
	})
	require.NoError(t, err)

	return body
}
