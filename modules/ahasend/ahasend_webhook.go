package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lunogram/platform/pkg/modules/providers"
)

// webhookTolerance is how far webhook-timestamp may drift from now, matching
// the AhaSend SDK default.
const webhookTolerance = 5 * time.Minute

type ahaSendWebhookPayload struct {
	Type      string         `json:"type"`
	Timestamp string         `json:"timestamp"`
	Data      map[string]any `json:"data"`
}

// mapWebhookEvent maps an AhaSend event type to a canonical WebhookEventName.
// Returns (eventName, ok).
func mapWebhookEvent(eventType string) (providers.WebhookEventName, bool) {
	switch eventType {
	case "message.reception":
		return providers.EventSent, true
	case "message.delivered":
		return providers.EventDelivered, true
	case "message.transient_error":
		return providers.EventDeferred, true
	case "message.bounced", "message.failed":
		return providers.EventBounced, true
	case "message.suppressed":
		return providers.EventDropped, true
	case "message.opened":
		return providers.EventOpened, true
	case "message.clicked":
		return providers.EventClicked, true
	case "message.complained":
		return providers.EventComplained, true
	case "message.unsubscribed":
		return providers.EventUnsubscribed, true
	default:
		return 0, false
	}
}

func parseAhaSendWebhookEvent(body []byte) (providers.WebhookEvent, bool, error) {
	var payload ahaSendWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return providers.WebhookEvent{}, false, fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	eventName, ok := mapWebhookEvent(payload.Type)
	if !ok {
		return providers.WebhookEvent{}, false, nil
	}

	messageID, _ := payload.Data["id"].(string)

	event := providers.WebhookEvent{
		EventName: eventName,
		MessageID: messageID,
		Timestamp: payload.Timestamp,
		Data:      payload.Data,
	}

	return event, true, nil
}

// verifyAhaSendWebhookSignature checks a Standard Webhooks signature. AhaSend
// uses the secret's literal bytes, prefix included, as the HMAC key; it is not
// base64-decoded as stock Standard Webhooks libraries do.
func verifyAhaSendWebhookSignature(secret string, headers map[string]string, body []byte) error {
	id := headers["webhook-id"]
	timestamp := headers["webhook-timestamp"]
	signatures := headers["webhook-signature"]
	if id == "" || timestamp == "" || signatures == "" {
		return errors.New("missing webhook signature headers")
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || ts <= 0 {
		return errors.New("invalid webhook timestamp")
	}
	if delta := time.Now().UTC().Sub(time.Unix(ts, 0).UTC()); delta > webhookTolerance || delta < -webhookTolerance {
		return errors.New("expired webhook timestamp")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(id + "." + timestamp + "."))
	_, _ = mac.Write(body)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	for _, signature := range strings.Fields(signatures) {
		version, value, found := strings.Cut(signature, ",")
		if found && version == "v1" && hmac.Equal([]byte(value), []byte(expected)) {
			return nil
		}
	}

	return errors.New("invalid webhook signature")
}
