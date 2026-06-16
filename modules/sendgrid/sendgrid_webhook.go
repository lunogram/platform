package main

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lunogram/platform/pkg/modules/providers"
)

// mapWebhookEvent maps a SendGrid event type string to a canonical
// WebhookEventName. Returns (eventName, ok).
func mapWebhookEvent(eventType string) (providers.WebhookEventName, bool) {
	switch eventType {
	case "processed":
		return providers.EventSent, true
	case "delivered":
		return providers.EventDelivered, true
	case "bounce", "blocked":
		return providers.EventBounced, true
	case "deferred":
		return providers.EventDeferred, true
	case "dropped":
		return providers.EventDropped, true
	case "open":
		return providers.EventOpened, true
	case "click":
		return providers.EventClicked, true
	case "spamreport":
		return providers.EventComplained, true
	case "unsubscribe", "group_unsubscribe":
		return providers.EventUnsubscribed, true
	default:
		return 0, false
	}
}

func parseSendGridWebhookEvents(body []byte) ([]providers.WebhookEvent, error) {
	var payload []map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		var single map[string]any
		if singleErr := json.Unmarshal(body, &single); singleErr != nil {
			return nil, err
		}
		payload = []map[string]any{single}
	}

	events := make([]providers.WebhookEvent, 0, len(payload))
	for _, rawEvent := range payload {
		eventType := getString(rawEvent, "event")
		eventName, ok := mapWebhookEvent(eventType)
		if !ok {
			continue
		}

		events = append(events, providers.WebhookEvent{
			EventName: eventName,
			MessageID: extractSendGridMessageID(rawEvent),
			Timestamp: parseSendGridTimestamp(rawEvent["timestamp"]),
			Data:      rawEvent,
		})
	}

	return events, nil
}

func getString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}

	s, ok := v.(string)
	if !ok {
		return ""
	}

	return s
}

func extractSendGridMessageID(event map[string]any) string {
	for _, key := range []string{"sg_message_id", "smtp-id", "smtp_id", "message_id"} {
		if value := getString(event, key); value != "" {
			return value
		}
	}

	return ""
}

func formatUnixTimestamp(unix int64) string {
	if unix <= 0 {
		return ""
	}

	return time.Unix(unix, 0).UTC().Format(time.RFC3339)
}

func parseSendGridTimestamp(raw any) string {
	switch v := raw.(type) {
	case float64:
		return formatUnixTimestamp(int64(v))
	case int64:
		return formatUnixTimestamp(v)
	case string:
		if v == "" {
			return ""
		}
		if unix, err := strconv.ParseInt(v, 10, 64); err == nil {
			return formatUnixTimestamp(unix)
		}
		if parsed, err := time.Parse(time.RFC3339, v); err == nil {
			return parsed.UTC().Format(time.RFC3339)
		}
	}

	return ""
}

const sendGridWebhookTimestampMaxSkew = 5 * time.Minute

// validateSendGridWebhookTimestamp checks if the provided timestamp header is a valid Unix timestamp
func validateSendGridWebhookTimestamp(timestampHeader string, now time.Time) error {
	unix, err := strconv.ParseInt(timestampHeader, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid SendGrid webhook timestamp: must be a Unix timestamp")
	}
	if unix <= 0 {
		return fmt.Errorf("invalid SendGrid webhook timestamp: must be a positive Unix timestamp")
	}
	timestamp := time.Unix(unix, 0).UTC()
	diff := now.UTC().Sub(timestamp)
	if diff < 0 {
		diff = -diff
	}
	if diff > sendGridWebhookTimestampMaxSkew {
		return fmt.Errorf("expired SendGrid webhook timestamp")
	}
	return nil
}

// parseSendGridVerificationKey parses the SendGrid Event Webhook verification
// key into an ECDSA public key. SendGrid exposes the key in the dashboard as a
// base64-encoded DER (PKIX) string, so that form is accepted directly; a
// PEM-armored key is also supported for convenience.
func parseSendGridVerificationKey(key string) (*ecdsa.PublicKey, error) {
	der := []byte(key)
	if block, _ := pem.Decode(der); block != nil {
		der = block.Bytes
	} else {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(key))
		if err != nil {
			return nil, fmt.Errorf("invalid SendGrid webhook verification key format")
		}
		der = decoded
	}

	publicKey, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SendGrid webhook verification key: %w", err)
	}

	ecdsaKey, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("SendGrid webhook verification key must be an ECDSA public key")
	}

	return ecdsaKey, nil
}

func verifySendGridWebhookSignature(publicKeyVerificationKey string, payload []byte, signatureHeader, timestampHeader string) error {
	if signatureHeader == "" || timestampHeader == "" {
		return fmt.Errorf("missing SendGrid signature headers")
	}

	if err := validateSendGridWebhookTimestamp(timestampHeader, time.Now()); err != nil {
		return err
	}

	ecdsaKey, err := parseSendGridVerificationKey(publicKeyVerificationKey)
	if err != nil {
		return err
	}

	signature, err := base64.StdEncoding.DecodeString(signatureHeader)
	if err != nil {
		return fmt.Errorf("failed to decode SendGrid webhook signature: %w", err)
	}

	signedPayload := make([]byte, 0, len(timestampHeader)+len(payload))
	signedPayload = append(signedPayload, timestampHeader...)
	signedPayload = append(signedPayload, payload...)
	digest := sha256.Sum256(signedPayload)

	if !ecdsa.VerifyASN1(ecdsaKey, digest[:], signature) {
		return fmt.Errorf("invalid SendGrid webhook signature")
	}

	return nil
}
