package payload

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lunogram/platform/pkg/modules/providers"
)

func TestBuild(t *testing.T) {
	t.Run("basic notification", func(t *testing.T) {
		got, err := Build(providers.PushPayload{Title: "Hello", Body: "World"}, nil)
		if err != nil {
			t.Fatalf("Build returned error: %v", err)
		}
		var out map[string]any
		if err := json.Unmarshal(got, &out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if out["title"] != "Hello" || out["body"] != "World" {
			t.Errorf("payload = %v, want title=Hello body=World", out)
		}
	})

	t.Run("metadata becomes top-level field", func(t *testing.T) {
		uuid := "11111111-2222-3333-4444-555555555555"
		got, err := Build(providers.PushPayload{Title: "T", Body: "B"}, map[string]string{
			"inbox_message_id": uuid,
		})
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		if !strings.Contains(string(got), uuid) {
			t.Errorf("payload missing UUID: %s", got)
		}
		var out map[string]any
		if err := json.Unmarshal(got, &out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if out["inbox_message_id"] != uuid {
			t.Errorf("inbox_message_id = %v, want %s", out["inbox_message_id"], uuid)
		}
	})

	t.Run("reserved keys cannot be overwritten", func(t *testing.T) {
		got, err := Build(providers.PushPayload{Title: "Real", Body: "Body"}, map[string]string{
			"title": "Spoofed",
			"body":  "Spoofed",
			"data":  "Spoofed",
			"image": "Spoofed",
			"badge": "Spoofed",
			"sound": "Spoofed",
		})
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		var out map[string]any
		if err := json.Unmarshal(got, &out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if out["title"] != "Real" {
			t.Errorf("title = %v, want Real", out["title"])
		}
		if out["body"] != "Body" {
			t.Errorf("body = %v, want Body", out["body"])
		}
		if _, ok := out["data"]; ok {
			t.Errorf("data should not be present when push.Data is empty: %v", out["data"])
		}
	})

	t.Run("badge and sound forwarded", func(t *testing.T) {
		badge := 7
		sound := "ping"
		got, err := Build(providers.PushPayload{Title: "T", Body: "B", Badge: &badge, Sound: &sound}, nil)
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		var out map[string]any
		if err := json.Unmarshal(got, &out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if out["badge"].(float64) != 7 {
			t.Errorf("badge = %v, want 7", out["badge"])
		}
		if out["sound"] != "ping" {
			t.Errorf("sound = %v, want ping", out["sound"])
		}
	})

	t.Run("empty metadata is no-op", func(t *testing.T) {
		got, err := Build(providers.PushPayload{Title: "T", Body: "B"}, map[string]string{})
		if err != nil {
			t.Fatalf("Build: %v", err)
		}
		var out map[string]any
		if err := json.Unmarshal(got, &out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(out) != 2 {
			t.Errorf("expected only title+body, got %v", out)
		}
	})
}
