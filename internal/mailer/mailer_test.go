package mailer

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

func TestRendererProducesBothParts(t *testing.T) {
	renderer, err := NewRenderer("Lunogram", "https://console.example.com/")
	if err != nil {
		t.Fatalf("failed to build renderer: %v", err)
	}

	tests := map[string]struct {
		message   Message
		wantURL   string
		wantEmpty bool
	}{
		"verify": {
			message: renderer.VerifyEmail("admin@example.com", "tok-1"),
			wantURL: "https://console.example.com/verify-email?token=tok-1",
		},
		"reset": {
			message: renderer.PasswordReset("admin@example.com", "tok-2", time.Hour),
			wantURL: "https://console.example.com/reset-password?token=tok-2",
		},
		"exists": {
			message: renderer.AccountExists("admin@example.com", "tok-3"),
			wantURL: "https://console.example.com/reset-password?token=tok-3",
		},
		"changed": {
			message:   renderer.PasswordChanged("admin@example.com"),
			wantEmpty: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if test.message.Subject == "" {
				t.Error("expected a subject")
			}
			if strings.TrimSpace(test.message.HTML) == "" {
				t.Error("expected an HTML part")
			}
			if strings.TrimSpace(test.message.Text) == "" {
				t.Error("expected a text part")
			}
			if test.wantEmpty {
				if test.message.ActionURL != "" {
					t.Errorf("expected no action URL, got %q", test.message.ActionURL)
				}
				return
			}
			if test.message.ActionURL != test.wantURL {
				t.Errorf("action URL = %q, want %q", test.message.ActionURL, test.wantURL)
			}
			if !strings.Contains(test.message.HTML, test.wantURL) {
				t.Error("expected the HTML part to carry the action URL")
			}
			if !strings.Contains(test.message.Text, test.wantURL) {
				t.Error("expected the text part to carry the action URL")
			}
		})
	}
}

// A token arriving in the query string must survive escaping, or the recipient
// follows a link that no longer names the token they were sent.
func TestRendererEscapesToken(t *testing.T) {
	renderer, err := NewRenderer("Lunogram", "https://console.example.com")
	if err != nil {
		t.Fatalf("failed to build renderer: %v", err)
	}

	message := renderer.PasswordReset("admin@example.com", "a+b/c=", time.Hour)
	if !strings.Contains(message.ActionURL, "token=a%2Bb%2Fc%3D") {
		t.Errorf("token was not escaped: %q", message.ActionURL)
	}
}

type recordingMailer struct {
	mu       sync.Mutex
	messages []Message
	err      error
	sent     chan struct{}
}

func (m *recordingMailer) Send(_ context.Context, message Message) error {
	m.mu.Lock()
	m.messages = append(m.messages, message)
	m.mu.Unlock()
	if m.sent != nil {
		m.sent <- struct{}{}
	}
	return m.err
}

func TestDispatcherDeliversAsynchronously(t *testing.T) {
	recorder := &recordingMailer{sent: make(chan struct{}, 1)}
	dispatcher := NewDispatcher(recorder, zaptest.NewLogger(t), time.Second)
	defer dispatcher.Close()

	dispatcher.Dispatch(Message{To: "admin@example.com", Subject: "hello"})

	select {
	case <-recorder.sent:
	case <-time.After(5 * time.Second):
		t.Fatal("message was never delivered")
	}
}

// A transport failure must stay inside the dispatcher: the handler that queued
// the message has already answered, and surfacing the failure would make the
// response depend on whether the address exists.
func TestDispatcherSwallowsTransportFailure(t *testing.T) {
	core, logs := observer.New(zap.ErrorLevel)
	recorder := &recordingMailer{err: errors.New("relay refused"), sent: make(chan struct{}, 1)}
	dispatcher := NewDispatcher(recorder, zap.New(core), time.Second)

	dispatcher.Dispatch(Message{To: "admin@example.com", Subject: "hello"})

	select {
	case <-recorder.sent:
	case <-time.After(5 * time.Second):
		t.Fatal("message was never attempted")
	}
	dispatcher.Close()

	if logs.FilterMessage("failed to deliver a message").Len() == 0 {
		t.Error("expected the failure to be logged")
	}
}

func TestDispatcherDrainsOnClose(t *testing.T) {
	recorder := &recordingMailer{}
	dispatcher := NewDispatcher(recorder, zaptest.NewLogger(t), time.Second)

	for i := range 10 {
		dispatcher.Dispatch(Message{To: "admin@example.com", Subject: string(rune('a' + i))})
	}
	dispatcher.Close()

	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if len(recorder.messages) != 10 {
		t.Errorf("delivered %d messages, want 10", len(recorder.messages))
	}
}

// The log transport is what a zero-configuration deployment falls back to, so
// the operator must be able to find the link in the log.
func TestLogMailerLogsTheActionURL(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	mailer := NewLogMailer(zap.New(core))

	err := mailer.Send(context.Background(), Message{
		To:        "admin@example.com",
		Subject:   "Confirm your email address",
		ActionURL: "https://console.example.com/verify-email?token=abc",
	})
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("logged %d entries, want 1", len(entries))
	}
	if url := entries[0].ContextMap()["action_url"]; url != "https://console.example.com/verify-email?token=abc" {
		t.Errorf("action_url = %v", url)
	}
}

func TestNewSMTPRejectsUnknownTLSMode(t *testing.T) {
	_, err := NewSMTP(Config{Host: "smtp.example.com", FromAddress: "no-reply@example.com", TLS: "maybe"})
	if err == nil {
		t.Fatal("expected an error for an unknown TLS mode")
	}
}

func TestNewFallsBackToLogMailer(t *testing.T) {
	built, err := New(Config{}, zaptest.NewLogger(t))
	if err != nil {
		t.Fatalf("failed to build mailer: %v", err)
	}
	if _, ok := built.(*LogMailer); !ok {
		t.Errorf("built %T, want *LogMailer", built)
	}
}
