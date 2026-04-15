package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"strconv"
	"testing"
	"time"

	"github.com/lunogram/platform/pkg/modules/providers"
	"github.com/stretchr/testify/assert"
)

func TestComposeSendGridMailRequest(t *testing.T) {
	t.Run("required fields with html and text", func(t *testing.T) {
		email := providers.EmailPayload{
			To:      "recipient@example.com",
			From:    providers.EmailAddress{Name: "Sender", Address: "sender@example.com"},
			Subject: "Test subject",
			HTML:    "<p>Hello</p>",
			Text:    "Hello",
		}

		req := ComposeSendEmailRequest(email)

		assert.Equal(t, "sender@example.com", req.From.Email)
		assert.Equal(t, "Sender", req.From.Name)
		assert.Equal(t, "Test subject", req.Subject)
		assert.Len(t, req.Personalizations, 1)
		assert.Equal(t, "recipient@example.com", req.Personalizations[0].To[0].Email)
		assert.Len(t, req.Content, 2)
		assert.Equal(t, "text/plain", req.Content[0].Type)
		assert.Equal(t, "text/html", req.Content[1].Type)
	})

	t.Run("optional fields are mapped", func(t *testing.T) {
		cc := "cc@example.com"
		bcc := "bcc@example.com"
		replyTo := "reply@example.com"

		email := providers.EmailPayload{
			To:      "recipient@example.com",
			From:    providers.EmailAddress{Address: "sender@example.com"},
			Subject: "Test",
			Text:    "Hello",
			Cc:      &cc,
			Bcc:     &bcc,
			ReplyTo: &replyTo,
			Headers: map[string]string{"X-Entity-Ref": "ref-123"},
		}

		req := ComposeSendEmailRequest(email)

		assert.Equal(t, "cc@example.com", req.Personalizations[0].Cc[0].Email)
		assert.Equal(t, "bcc@example.com", req.Personalizations[0].Bcc[0].Email)
		assert.Equal(t, "reply@example.com", req.ReplyTo.Email)
		assert.Equal(t, "ref-123", req.Personalizations[0].Headers["X-Entity-Ref"])
	})
}

func TestMapWebhookEvent(t *testing.T) {
	tests := []struct {
		name          string
		eventType     string
		expectedEvent providers.WebhookEventName
		expectedOK    bool
	}{
		{name: "processed", eventType: "processed", expectedEvent: providers.EventSent, expectedOK: true},
		{name: "delivered", eventType: "delivered", expectedEvent: providers.EventDelivered, expectedOK: true},
		{name: "bounce", eventType: "bounce", expectedEvent: providers.EventBounced, expectedOK: true},
		{name: "blocked", eventType: "blocked", expectedEvent: providers.EventBounced, expectedOK: true},
		{name: "deferred", eventType: "deferred", expectedEvent: providers.EventDeferred, expectedOK: true},
		{name: "dropped", eventType: "dropped", expectedEvent: providers.EventDropped, expectedOK: true},
		{name: "open", eventType: "open", expectedEvent: providers.EventOpened, expectedOK: true},
		{name: "click", eventType: "click", expectedEvent: providers.EventClicked, expectedOK: true},
		{name: "spamreport", eventType: "spamreport", expectedEvent: providers.EventComplained, expectedOK: true},
		{name: "unsubscribe", eventType: "unsubscribe", expectedEvent: providers.EventUnsubscribed, expectedOK: true},
		{name: "group_unsubscribe", eventType: "group_unsubscribe", expectedEvent: providers.EventUnsubscribed, expectedOK: true},
		{name: "unknown event", eventType: "unknown", expectedEvent: 0, expectedOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventName, ok := mapWebhookEvent(tt.eventType)
			assert.Equal(t, tt.expectedOK, ok)
			assert.Equal(t, tt.expectedEvent, eventName)
		})
	}
}

func TestWebhookEvents(t *testing.T) {
	events := webhookEvents()

	assert.Contains(t, events, "processed")
	assert.Contains(t, events, "delivered")
	assert.Contains(t, events, "bounce")
	assert.Contains(t, events, "blocked")
	assert.Contains(t, events, "deferred")
	assert.Contains(t, events, "dropped")
	assert.Contains(t, events, "open")
	assert.Contains(t, events, "click")
	assert.Contains(t, events, "spamreport")
	assert.Contains(t, events, "unsubscribe")
	assert.Contains(t, events, "group_unsubscribe")
	assert.Len(t, events, 11)
}

func TestParseSendGridWebhookEvents(t *testing.T) {
	body := []byte(`[
		{"event":"processed","sg_message_id":"msg-1","timestamp":1710000000,"email":"a@example.com"},
		{"event":"delivered","sg_message_id":"msg-2","timestamp":1710000030,"email":"b@example.com"},
		{"event":"unknown","sg_message_id":"msg-3","timestamp":1710000060}
	]`)

	events, err := parseSendGridWebhookEvents(body)
	assert.NoError(t, err)
	assert.Len(t, events, 2)

	assert.Equal(t, providers.EventSent, events[0].EventName)
	assert.Equal(t, "msg-1", events[0].MessageID)
	assert.Equal(t, time.Unix(1710000000, 0).UTC().Format(time.RFC3339), events[0].Timestamp)
	assert.Equal(t, "a@example.com", events[0].Data["email"])

	assert.Equal(t, providers.EventDelivered, events[1].EventName)
	assert.Equal(t, "msg-2", events[1].MessageID)
	assert.Equal(t, time.Unix(1710000030, 0).UTC().Format(time.RFC3339), events[1].Timestamp)
	assert.Equal(t, "b@example.com", events[1].Data["email"])
}

func TestVerifySendGridWebhookSignature(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	assert.NoError(t, err)

	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	assert.NoError(t, err)

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyDER})

	payload := []byte(`[{"event":"delivered","sg_message_id":"msg-1"}]`)
	timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)

	signedPayload := make([]byte, 0, len(timestamp)+len(payload))
	signedPayload = append(signedPayload, timestamp...)
	signedPayload = append(signedPayload, payload...)
	digest := sha256.Sum256(signedPayload)

	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	assert.NoError(t, err)

	signatureHeader := base64.StdEncoding.EncodeToString(signature)

	err = verifySendGridWebhookSignature(string(publicKeyPEM), payload, signatureHeader, timestamp)
	assert.NoError(t, err)

	invalidTimestamp := strconv.FormatInt(time.Now().UTC().Unix()+1, 10)
	err = verifySendGridWebhookSignature(string(publicKeyPEM), payload, signatureHeader, invalidTimestamp)
	assert.Error(t, err)

	err = verifySendGridWebhookSignature(string(publicKeyPEM), payload, "", timestamp)
	assert.Error(t, err)
}

func TestClassifyHTTPStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		expected int32
	}{
		{name: "400 permanent", status: 400, expected: ExitPermanent},
		{name: "401 permanent", status: 401, expected: ExitPermanent},
		{name: "422 permanent", status: 422, expected: ExitPermanent},
		{name: "429 transient", status: 429, expected: ExitTransient},
		{name: "500 transient", status: 500, expected: ExitTransient},
		{name: "503 transient", status: 503, expected: ExitTransient},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, classifyHTTPStatus(tt.status))
		})
	}
}

func TestValidateConfig(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		err := validateConfig(Config{APIKey: "SG.key"})
		assert.Empty(t, err)
	})

	t.Run("missing api key", func(t *testing.T) {
		err := validateConfig(Config{})
		assert.Equal(t, "API key is required", err["apiKey"])
	})
}

func TestFormatSendGridErrors(t *testing.T) {
	body := sendGridErrorBody{}
	body.Errors = []struct {
		Message string `json:"message"`
		Field   string `json:"field"`
		Help    string `json:"help"`
	}{
		{Message: "invalid email", Field: "personalizations.0.to.0.email", Help: "https://docs.example"},
	}

	msg := formatSendGridErrors(body)
	assert.Contains(t, msg, "invalid email")
	assert.Contains(t, msg, "field=personalizations.0.to.0.email")
}
