package payload

import (
	"encoding/json"
	"testing"

	"github.com/lunogram/platform/pkg/modules/providers"
)

func TestBuild(t *testing.T) {
	t.Run("basic payload contains aps alert", func(t *testing.T) {
		raw, err := Build(providers.PushPayload{
			Title: "hello",
			Body:  "world",
		}, nil)
		if err != nil {
			t.Fatalf("Build returned error: %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		aps, ok := got["aps"].(map[string]any)
		if !ok {
			t.Fatalf("missing aps key, got %T: %v", got["aps"], got)
		}
		alert, ok := aps["alert"].(map[string]any)
		if !ok {
			t.Fatalf("missing aps.alert: %v", aps)
		}
		if alert["title"] != "hello" || alert["body"] != "world" {
			t.Fatalf("alert mismatch: %v", alert)
		}
	})

	t.Run("inbox_message_id metadata is emitted as top-level key", func(t *testing.T) {
		uuid := "550e8400-e29b-41d4-a716-446655440000"
		raw, err := Build(providers.PushPayload{Title: "t", Body: "b"}, map[string]string{
			providers.MetadataKeyInboxMessageID: uuid,
		})
		if err != nil {
			t.Fatalf("Build returned error: %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if got[providers.MetadataKeyInboxMessageID] != uuid {
			t.Fatalf("expected top-level inbox_message_id=%q, got %v", uuid, got)
		}
		if _, ok := got["aps"]; !ok {
			t.Fatalf("aps key clobbered: %v", got)
		}
	})

	t.Run("metadata cannot overwrite reserved keys", func(t *testing.T) {
		raw, err := Build(providers.PushPayload{
			Title: "t",
			Body:  "b",
			Data:  map[string]any{"foo": "bar"},
		}, map[string]string{
			"aps":  "evil",
			"data": "evil",
		})
		if err != nil {
			t.Fatalf("Build returned error: %v", err)
		}

		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		if _, ok := got["aps"].(map[string]any); !ok {
			t.Fatalf("aps not preserved: %T", got["aps"])
		}
		if _, ok := got["data"].(map[string]any); !ok {
			t.Fatalf("data not preserved: %T", got["data"])
		}
	})

	t.Run("badge and sound are forwarded", func(t *testing.T) {
		badge := 5
		sound := "default"
		raw, err := Build(providers.PushPayload{
			Title: "t",
			Body:  "b",
			Badge: &badge,
			Sound: &sound,
		}, nil)
		if err != nil {
			t.Fatalf("Build returned error: %v", err)
		}

		var got struct {
			APS struct {
				Badge *int    `json:"badge"`
				Sound *string `json:"sound"`
			} `json:"aps"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.APS.Badge == nil || *got.APS.Badge != 5 {
			t.Fatalf("badge mismatch: %+v", got.APS.Badge)
		}
		if got.APS.Sound == nil || *got.APS.Sound != "default" {
			t.Fatalf("sound mismatch: %+v", got.APS.Sound)
		}
	})

	t.Run("nil and empty metadata produce no extra keys", func(t *testing.T) {
		raw, err := Build(providers.PushPayload{Title: "t", Body: "b"}, map[string]string{})
		if err != nil {
			t.Fatalf("Build returned error: %v", err)
		}
		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("expected only aps key, got %v", got)
		}
	})
}
