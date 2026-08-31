package mailer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lunogram/platform/internal/configfile"
	"github.com/lunogram/platform/internal/outbound"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"go.uber.org/zap/zaptest/observer"
)

func testRenderer(t *testing.T, config Config, baseDir string) *Renderer {
	t.Helper()
	renderer, err := NewRenderer(config, "https://console.example.com/", baseDir)
	if err != nil {
		t.Fatalf("failed to build renderer: %v", err)
	}
	return renderer
}

func TestRendererProducesBothParts(t *testing.T) {
	renderer := testRenderer(t, DefaultConfig(), "")

	tests := map[string]struct {
		message   Message
		wantURL   string
		wantEmpty bool
	}{
		"verify": {
			message: renderer.VerifyEmail("admin@example.com", "tok-1", 24*time.Hour),
			wantURL: "https://console.example.com/verify-email?token=tok-1",
		},
		"reset": {
			message: renderer.PasswordReset("admin@example.com", "tok-2", time.Hour),
			wantURL: "https://console.example.com/reset-password?token=tok-2",
		},
		"exists": {
			message: renderer.AccountExists("admin@example.com", "tok-3", time.Hour),
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
			if test.message.Kind == "" {
				t.Error("expected the message to name its kind")
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
	renderer := testRenderer(t, DefaultConfig(), "")

	message := renderer.PasswordReset("admin@example.com", "a+b/c=", time.Hour)
	if !strings.Contains(message.ActionURL, "token=a%2Bb%2Fc%3D") {
		t.Errorf("token was not escaped: %q", message.ActionURL)
	}
}

// A deployment that configures no templates still sends the messages this
// package ships, which is what keeps the mail section optional.
func TestRendererFallsBackToEmbeddedDefaults(t *testing.T) {
	message := testRenderer(t, DefaultConfig(), "").VerifyEmail("admin@example.com", "tok", 24*time.Hour)

	if message.Subject != "Confirm your email address" {
		t.Errorf("subject = %q", message.Subject)
	}
	if !strings.Contains(message.Text, "1 day") {
		t.Error("expected the expiry to be written out for a human")
	}
}

func TestRendererAppliesLiteralOverride(t *testing.T) {
	config := DefaultConfig()
	config.Templates.VerifyEmail.Subject = "Please confirm your address"
	config.Templates.VerifyEmail.Body = []string{"One line, from {{ .ProductName }}."}

	message := testRenderer(t, config, "").VerifyEmail("admin@example.com", "tok", time.Hour)

	if message.Subject != "Please confirm your address" {
		t.Errorf("subject = %q", message.Subject)
	}
	if !strings.Contains(message.Text, "One line, from Lunogram.") {
		t.Errorf("body was not overridden: %q", message.Text)
	}
}

// An override the operator did not set falls back per field, so changing a
// subject does not mean restating the body.
func TestRendererOverridesOneFieldAtATime(t *testing.T) {
	config := DefaultConfig()
	config.Templates.PasswordReset.Subject = "Set a new password"

	message := testRenderer(t, config, "").PasswordReset("admin@example.com", "tok", time.Hour)

	if message.Subject != "Set a new password" {
		t.Errorf("subject = %q", message.Subject)
	}
	if !strings.Contains(message.Text, "Choose a new password") {
		t.Error("expected the default action label to survive a subject override")
	}
}

func TestRendererAppliesBase64Override(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("Confirm {{ .Recipient }}"))

	config := DefaultConfig()
	config.Templates.VerifyEmail.Subject = configfile.Base64Scheme + encoded

	message := testRenderer(t, config, "").VerifyEmail("admin@example.com", "tok", time.Hour)

	if message.Subject != "Confirm admin@example.com" {
		t.Errorf("subject = %q", message.Subject)
	}
}

func TestRendererAppliesFileOverride(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "layout.html"), []byte("<p>{{ .Heading }}</p>"), 0o600); err != nil {
		t.Fatal(err)
	}

	config := DefaultConfig()
	config.Templates.Layout.HTML = configfile.FileScheme + "layout.html"

	message := testRenderer(t, config, dir).VerifyEmail("admin@example.com", "tok", time.Hour)

	if message.HTML != "<p>Confirm your email address</p>" {
		t.Errorf("HTML = %q", message.HTML)
	}
}

// An absent override is a deployment that did not ask for anything. A present
// one that cannot be resolved is a deployment that asked for something specific
// and got it wrong, and falling back there would send the wrong mail with
// nothing in the logs to explain why.
func TestRendererRejectsBrokenOverrides(t *testing.T) {
	tests := map[string]func(*Config){
		"missing file": func(c *Config) {
			c.Templates.Layout.HTML = configfile.FileScheme + "absent.tmpl"
		},
		"invalid base64": func(c *Config) {
			c.Templates.VerifyEmail.Subject = configfile.Base64Scheme + "not!valid!"
		},
		"unparsable template": func(c *Config) {
			c.Templates.VerifyEmail.Subject = "{{ .Unclosed"
		},
		"unparsable layout": func(c *Config) {
			c.Templates.Layout.Text = "{{ range .Body }}"
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			config := DefaultConfig()
			mutate(&config)

			if _, err := NewRenderer(config, "https://console.example.com", t.TempDir()); err == nil {
				t.Fatal("expected the renderer to refuse a broken override")
			}
		})
	}
}

// The copy is escaped by the HTML layout, so a value that reaches it from a
// request cannot inject markup into somebody's mailbox.
func TestRendererEscapesCopyInHTML(t *testing.T) {
	config := DefaultConfig()
	config.ProductName = "<script>alert(1)</script>"

	message := testRenderer(t, config, "").VerifyEmail("admin@example.com", "tok", time.Hour)

	if strings.Contains(message.HTML, "<script>") {
		t.Errorf("product name was not escaped: %q", message.HTML)
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

func TestNewSMTPRejectsUnknownTLSMode(t *testing.T) {
	config := DefaultConfig()
	config.SMTP.Host = "smtp.example.com"
	config.SMTP.TLS = "maybe"
	config.From.Address = "no-reply@example.com"

	if _, err := NewSMTP(config); err == nil {
		t.Fatal("expected an error for an unknown TLS mode")
	}
}

// There is no channel that quietly swallows mail: a deployment offering
// password logins has to say where its mail goes, and one that does not is
// refused at boot rather than at the first registration.
func TestNewRequiresAChannel(t *testing.T) {
	_, err := New(DefaultConfig(), "", zaptest.NewLogger(t))
	if err == nil {
		t.Fatal("expected an error when no channel is configured")
	}
	if !strings.Contains(err.Error(), "no channel is configured") {
		t.Errorf("error did not say what to set: %v", err)
	}
}

func TestNewRejectsUnknownChannel(t *testing.T) {
	config := DefaultConfig()
	config.Channel = "carrier-pigeon"

	if _, err := New(config, "", zaptest.NewLogger(t)); err == nil {
		t.Fatal("expected an error for an unknown channel")
	}
}

func webhookConfig(url string) Config {
	config := DefaultConfig()
	config.Channel = ChannelWebhook
	config.From = From{Address: "no-reply@example.com", Name: "Lunogram"}
	config.Webhook.URL = url
	config.Webhook.Timeout = 5 * time.Second
	config.Webhook.network = outbound.Network{AllowPrivate: true, AllowHTTP: true}
	return config
}

// The default body produces the shape Mailpit's send API accepts, which is what
// the compose quickstart points at.
func TestWebhookChannelPostsTheRenderedMessage(t *testing.T) {
	received := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var decoded map[string]any
		if err := json.Unmarshal(body, &decoded); err != nil {
			t.Errorf("body was not JSON: %v", err)
		}
		received <- decoded
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	channel, err := New(webhookConfig(server.URL), "", zaptest.NewLogger(t))
	if err != nil {
		t.Fatalf("failed to build the webhook channel: %v", err)
	}

	message := testRenderer(t, DefaultConfig(), "").VerifyEmail("admin@example.com", "tok", time.Hour)
	if err := channel.Send(context.Background(), message); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	body := <-received
	if body["Subject"] != "Confirm your email address" {
		t.Errorf("Subject = %v", body["Subject"])
	}
	if from, ok := body["From"].(map[string]any); !ok || from["Email"] != "no-reply@example.com" {
		t.Errorf("From = %v", body["From"])
	}
	to, ok := body["To"].([]any)
	if !ok || len(to) != 1 {
		t.Fatalf("To = %v", body["To"])
	}
	if recipient := to[0].(map[string]any); recipient["Email"] != "admin@example.com" {
		t.Errorf("recipient = %v", recipient)
	}
}

// A deployment sending through its own service overrides the body template;
// nothing else has to change.
func TestWebhookChannelUsesAConfiguredBody(t *testing.T) {
	received := make(chan map[string]any, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var decoded map[string]any
		_ = json.Unmarshal(body, &decoded)
		received <- decoded
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	config := webhookConfig(server.URL)
	config.Webhook.Body = `function(ctx) { recipient: ctx.message.to, kind: ctx.kind, link: ctx.message.action_url }`

	channel, err := New(config, "", zaptest.NewLogger(t))
	if err != nil {
		t.Fatalf("failed to build the webhook channel: %v", err)
	}

	message := testRenderer(t, DefaultConfig(), "").PasswordReset("admin@example.com", "tok", time.Hour)
	if err := channel.Send(context.Background(), message); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	body := <-received
	if body["recipient"] != "admin@example.com" {
		t.Errorf("recipient = %v", body["recipient"])
	}
	if body["kind"] != KindPasswordReset {
		t.Errorf("kind = %v", body["kind"])
	}
	if body["link"] == "" {
		t.Error("expected the action URL to reach the template")
	}
}

func TestWebhookChannelReportsRejection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	config := webhookConfig(server.URL)
	config.Webhook.Retry = &outbound.Retry{MaxAttempts: 1}

	channel, err := New(config, "", zaptest.NewLogger(t))
	if err != nil {
		t.Fatalf("failed to build the webhook channel: %v", err)
	}

	message := testRenderer(t, DefaultConfig(), "").PasswordChanged("admin@example.com")
	if err := channel.Send(context.Background(), message); err == nil {
		t.Fatal("expected a rejected delivery to be reported")
	}
}

func TestWebhookChannelRequiresAURL(t *testing.T) {
	config := DefaultConfig()
	config.Channel = ChannelWebhook
	config.From.Address = "no-reply@example.com"

	if _, err := New(config, "", zaptest.NewLogger(t)); err == nil {
		t.Fatal("expected an error when no webhook URL is configured")
	}
}
