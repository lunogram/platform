package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lunogram/platform/pkg/modules/providers"
)

type mailgunWebhookPayload struct {
	Signature struct {
		TimeStamp string `json:"timestamp"`
		Token     string `json:"token"`
		Signature string `json:"signature"`
	} `json:"signature"`
	EventData json.RawMessage `json:"event-data"`
}

// mapWebhookEvent maps a Mailgun event type string to a canonical
// WebhookEventName. Returns (eventName, ok).
func mapWebhookEvent(eventType string) (providers.WebhookEventName, bool) {
	switch eventType {
	case "accepted":
		return providers.EventSent, true
	case "delivered":
		return providers.EventDelivered, true
	case "opened":
		return providers.EventOpened, true
	case "clicked":
		return providers.EventClicked, true
	case "complained":
		return providers.EventComplained, true
	case "unsubscribed":
		return providers.EventUnsubscribed, true
	case "failed":
		return providers.EventBounced, true
	default:
		return 0, false
	}
}

// webhookEvents returns the list of Mailgun webhook event types this provider uses.
func webhookEvents() []string {
	return []string{
		"accepted",
		"delivered",
		"failed",
		"opened",
		"clicked",
		"complained",
		"unsubscribed",
	}
}

func parseMailgunWebhookEvent(body []byte) (providers.WebhookEvent, bool, error) {
	var payload mailgunWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return providers.WebhookEvent{}, false, fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	var eventData map[string]any
	if err := json.Unmarshal(payload.EventData, &eventData); err != nil {
		return providers.WebhookEvent{}, false, fmt.Errorf("failed to parse webhook event-data: %w", err)
	}

	eventType, _ := eventData["event"].(string)
	eventName, ok := mapWebhookEvent(eventType)
	if ok && eventType == "failed" {
		if deliveryStatus, ok := eventData["delivery-status"].(map[string]any); ok {
			if severity, ok := deliveryStatus["severity"].(string); ok && strings.EqualFold(severity, "temporary") {
				eventName = providers.EventDeferred
			}
		}
	}
	if !ok {
		return providers.WebhookEvent{}, false, nil
	}

	messageID, _ := eventData["id"].(string)
	timestampISO := ""
	if ts, ok := eventData["timestamp"].(float64); ok && ts > 0 {
		timestampISO = time.Unix(int64(ts), 0).UTC().Format(time.RFC3339)
	}

	event := providers.WebhookEvent{
		EventName: eventName,
		MessageID: messageID,
		Timestamp: timestampISO,
		Data:      eventData,
	}

	return event, true, nil
}

func verifyMailgunWebhookSignature(signingKey string, body []byte) error {
	var payload mailgunWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("failed to parse webhook payload for signature verification: %w", err)
	}

	timestamp := payload.Signature.TimeStamp
	token := payload.Signature.Token
	signature := payload.Signature.Signature
	if timestamp == "" || token == "" || signature == "" {
		return errors.New("missing webhook signature fields")
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || ts <= 0 {
		return errors.New("invalid webhook timestamp")
	}
	if delta := time.Now().UTC().Sub(time.Unix(ts, 0).UTC()); delta > 15*time.Minute || delta < -15*time.Minute {
		return errors.New("expired webhook timestamp")
	}

	mac := hmac.New(sha256.New, []byte(signingKey))
	_, _ = mac.Write([]byte(timestamp + token))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return errors.New("invalid webhook signature")
	}
	return nil
}
