package main

import (
	"errors"
	"testing"

	"github.com/lunogram/platform/pkg/modules/providers"
	"github.com/resend/resend-go/v3"
	"github.com/stretchr/testify/assert"
)

func TestFormatAddress(t *testing.T) {
	tests := []struct {
		name     string
		address  providers.EmailAddress
		expected string
	}{
		{
			name:     "address only",
			address:  providers.EmailAddress{Address: "user@example.com"},
			expected: "user@example.com",
		},
		{
			name:     "name and address",
			address:  providers.EmailAddress{Name: "John Doe", Address: "john@example.com"},
			expected: "John Doe <john@example.com>",
		},
		{
			name:     "empty name uses address only",
			address:  providers.EmailAddress{Name: "", Address: "test@example.com"},
			expected: "test@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAddress(tt.address)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected int32
	}{
		{
			name:     "nil error returns success",
			err:      nil,
			expected: 0,
		},
		{
			name:     "rate limit error is transient",
			err:      &resend.RateLimitError{Message: "rate limit exceeded"},
			expected: ExitTransient,
		},
		{
			name:     "validation error is permanent",
			err:      errors.New("[ERROR]: validation_error: missing required field"),
			expected: ExitPermanent,
		},
		{
			name:     "missing field error is permanent",
			err:      errors.New("[ERROR]: missing required 'to' field"),
			expected: ExitPermanent,
		},
		{
			name:     "invalid input error is permanent",
			err:      errors.New("[ERROR]: invalid email address"),
			expected: ExitPermanent,
		},
		{
			name:     "generic server error is transient",
			err:      errors.New("[ERROR]: Internal Server Error"),
			expected: ExitTransient,
		},
		{
			name:     "unknown error is transient",
			err:      errors.New("something unexpected happened"),
			expected: ExitTransient,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMapWebhookEvent(t *testing.T) {
	tests := []struct {
		name          string
		eventType     string
		expectedEvent providers.WebhookEventName
		expectedOK    bool
	}{
		{
			name:          "email.sent",
			eventType:     resend.EventEmailSent,
			expectedEvent: providers.EventSent,
			expectedOK:    true,
		},
		{
			name:          "email.delivered",
			eventType:     resend.EventEmailDelivered,
			expectedEvent: providers.EventDelivered,
			expectedOK:    true,
		},
		{
			name:          "email.opened",
			eventType:     resend.EventEmailOpened,
			expectedEvent: providers.EventOpened,
			expectedOK:    true,
		},
		{
			name:          "email.clicked",
			eventType:     resend.EventEmailClicked,
			expectedEvent: providers.EventClicked,
			expectedOK:    true,
		},
		{
			name:          "email.bounced",
			eventType:     resend.EventEmailBounced,
			expectedEvent: providers.EventBounced,
			expectedOK:    true,
		},
		{
			name:          "email.complained",
			eventType:     resend.EventEmailComplained,
			expectedEvent: providers.EventComplained,
			expectedOK:    true,
		},
		{
			name:          "email.delivery_delayed",
			eventType:     resend.EventEmailDeliveryDelayed,
			expectedEvent: providers.EventDeferred,
			expectedOK:    true,
		},
		{
			name:          "unknown event type",
			eventType:     "email.unknown",
			expectedEvent: 0,
			expectedOK:    false,
		},
		{
			name:          "contact event is not mapped",
			eventType:     resend.EventContactCreated,
			expectedEvent: 0,
			expectedOK:    false,
		},
		{
			name:          "empty string",
			eventType:     "",
			expectedEvent: 0,
			expectedOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, ok := mapWebhookEvent(tt.eventType)
			assert.Equal(t, tt.expectedOK, ok)
			assert.Equal(t, tt.expectedEvent, event)
		})
	}
}

func TestBuildSendEmailRequest(t *testing.T) {
	t.Run("basic email with required fields", func(t *testing.T) {
		email := providers.EmailPayload{
			To:      "recipient@example.com",
			From:    providers.EmailAddress{Address: "sender@example.com"},
			Subject: "Test Subject",
			HTML:    "<p>Hello</p>",
			Text:    "Hello",
		}

		req := ComposeSendEmailRequest(email)

		assert.Equal(t, "sender@example.com", req.From)
		assert.Equal(t, []string{"recipient@example.com"}, req.To)
		assert.Equal(t, "Test Subject", req.Subject)
		assert.Equal(t, "<p>Hello</p>", req.Html)
		assert.Equal(t, "Hello", req.Text)
		assert.Nil(t, req.Cc)
		assert.Nil(t, req.Bcc)
		assert.Empty(t, req.ReplyTo)
	})

	t.Run("email with from name", func(t *testing.T) {
		email := providers.EmailPayload{
			To:      "recipient@example.com",
			From:    providers.EmailAddress{Name: "Sender Name", Address: "sender@example.com"},
			Subject: "Test",
		}

		req := ComposeSendEmailRequest(email)

		assert.Equal(t, "Sender Name <sender@example.com>", req.From)
	})

	t.Run("email with cc bcc and reply_to", func(t *testing.T) {
		cc := "cc@example.com"
		bcc := "bcc@example.com"
		replyTo := "reply@example.com"

		email := providers.EmailPayload{
			To:      "recipient@example.com",
			From:    providers.EmailAddress{Address: "sender@example.com"},
			Subject: "Test",
			Cc:      &cc,
			Bcc:     &bcc,
			ReplyTo: &replyTo,
		}

		req := ComposeSendEmailRequest(email)

		assert.Equal(t, []string{"cc@example.com"}, req.Cc)
		assert.Equal(t, []string{"bcc@example.com"}, req.Bcc)
		assert.Equal(t, "reply@example.com", req.ReplyTo)
	})

	t.Run("email with custom headers", func(t *testing.T) {
		email := providers.EmailPayload{
			To:      "recipient@example.com",
			From:    providers.EmailAddress{Address: "sender@example.com"},
			Subject: "Test",
			Headers: map[string]string{
				"X-Custom-Header": "custom-value",
				"X-Entity-Ref":    "ref-123",
			},
		}

		req := ComposeSendEmailRequest(email)

		assert.Equal(t, "custom-value", req.Headers["X-Custom-Header"])
		assert.Equal(t, "ref-123", req.Headers["X-Entity-Ref"])
	})

	t.Run("email with nil optional fields", func(t *testing.T) {
		email := providers.EmailPayload{
			To:      "recipient@example.com",
			From:    providers.EmailAddress{Address: "sender@example.com"},
			Subject: "Test",
			Cc:      nil,
			Bcc:     nil,
			ReplyTo: nil,
		}

		req := ComposeSendEmailRequest(email)

		assert.Nil(t, req.Cc)
		assert.Nil(t, req.Bcc)
		assert.Empty(t, req.ReplyTo)
	})
}

func TestWebhookEvents(t *testing.T) {
	events := webhookEvents()

	assert.Contains(t, events, resend.EventEmailSent)
	assert.Contains(t, events, resend.EventEmailDelivered)
	assert.Contains(t, events, resend.EventEmailBounced)
	assert.Contains(t, events, resend.EventEmailOpened)
	assert.Contains(t, events, resend.EventEmailClicked)
	assert.Contains(t, events, resend.EventEmailComplained)
	assert.Len(t, events, 6)
}

func TestExitCodes(t *testing.T) {
	assert.Equal(t, int32(-1), ExitTransient)
	assert.Equal(t, int32(-2), ExitPermanent)
}
