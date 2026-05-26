package payload

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lunogram/platform/pkg/modules/providers"
)

func TestBuild(t *testing.T) {
	t.Run("basic notification", func(t *testing.T) {
		got, err := Build("token-1", providers.PushPayload{
			Title: "Hello",
			Body:  "World",
		}, nil)
		if err != nil {
			t.Fatalf("Build returned error: %v", err)
		}

		var msg Message
		if err := json.Unmarshal(got, &msg); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if msg.Message.Token != "token-1" {
			t.Errorf("token = %q, want %q", msg.Message.Token, "token-1")
		}
		if msg.Message.Notification == nil || msg.Message.Notification.Title != "Hello" || msg.Message.Notification.Body != "World" {
			t.Errorf("notification = %+v, want title=Hello body=World", msg.Message.Notification)
		}
		if msg.Message.Data != nil {
			t.Errorf("data = %v, want nil", msg.Message.Data)
		}
	})

	t.Run("metadata becomes data field", func(t *testing.T) {
		uuid := "11111111-2222-3333-4444-555555555555"
		got, err := Build("token-1", providers.PushPayload{Title: "T", Body: "B"}, map[string]string{
			"inbox_message_id": uuid,
		})
		if err != nil {
			t.Fatalf("Build returned error: %v", err)
		}

		if !strings.Contains(string(got), uuid) {
			t.Errorf("payload missing UUID: %s", got)
		}

		var msg Message
		if err := json.Unmarshal(got, &msg); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if msg.Message.Data["inbox_message_id"] != uuid {
			t.Errorf("data[inbox_message_id] = %q, want %q", msg.Message.Data["inbox_message_id"], uuid)
		}
	})

	t.Run("push data takes precedence over metadata", func(t *testing.T) {
		got, err := Build("token-1", providers.PushPayload{
			Title: "T",
			Body:  "B",
			Data:  map[string]any{"k": "from-push"},
		}, map[string]string{
			"k":                "from-metadata",
			"inbox_message_id": "u",
		})
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		var msg Message
		if err := json.Unmarshal(got, &msg); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if msg.Message.Data["k"] != "from-push" {
			t.Errorf("data[k] = %q, want from-push", msg.Message.Data["k"])
		}
		if msg.Message.Data["inbox_message_id"] != "u" {
			t.Errorf("data[inbox_message_id] = %q, want u", msg.Message.Data["inbox_message_id"])
		}
	})

	t.Run("sound forwards to android and apns", func(t *testing.T) {
		s := "ping"
		got, err := Build("t", providers.PushPayload{Title: "T", Body: "B", Sound: &s}, nil)
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		var msg Message
		if err := json.Unmarshal(got, &msg); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if msg.Message.Android == nil || msg.Message.Android.Notification == nil || msg.Message.Android.Notification.Sound != "ping" {
			t.Errorf("android sound not set: %+v", msg.Message.Android)
		}
		if msg.Message.APNS == nil || msg.Message.APNS.Payload == nil || msg.Message.APNS.Payload.APS == nil || msg.Message.APNS.Payload.APS.Sound != "ping" {
			t.Errorf("apns sound not set: %+v", msg.Message.APNS)
		}
	})

	t.Run("empty metadata leaves data nil", func(t *testing.T) {
		got, err := Build("t", providers.PushPayload{Title: "T", Body: "B"}, map[string]string{})
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		var msg Message
		if err := json.Unmarshal(got, &msg); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if msg.Message.Data != nil {
			t.Errorf("data = %v, want nil", msg.Message.Data)
		}
	})
}
