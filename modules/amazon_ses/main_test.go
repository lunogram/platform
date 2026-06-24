package main

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/lunogram/platform/pkg/modules/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComposeSendEmailRequest(t *testing.T) {
	t.Run("required fields with html and text", func(t *testing.T) {
		email := providers.EmailPayload{
			To:      "recipient@example.com",
			From:    providers.EmailAddress{Name: "Sender", Address: "sender@example.com"},
			Subject: "Test subject",
			HTML:    "<p>Hello</p>",
			Text:    "Hello",
		}

		req := ComposeSendEmailRequest(email, Config{})

		assert.Equal(t, "Sender <sender@example.com>", req.FromEmailAddress)
		assert.Equal(t, []string{"recipient@example.com"}, req.Destination.ToAddresses)
		assert.Equal(t, "Test subject", req.Content.Simple.Subject.Data)
		require.NotNil(t, req.Content.Simple.Body.Html)
		assert.Equal(t, "<p>Hello</p>", req.Content.Simple.Body.Html.Data)
		require.NotNil(t, req.Content.Simple.Body.Text)
		assert.Equal(t, "Hello", req.Content.Simple.Body.Text.Data)
		assert.Nil(t, req.ConfigurationSetName)
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
		}

		req := ComposeSendEmailRequest(email, Config{ConfigurationSet: "my-set"})

		assert.Equal(t, "sender@example.com", req.FromEmailAddress)
		assert.Equal(t, []string{"cc@example.com"}, req.Destination.CcAddresses)
		assert.Equal(t, []string{"bcc@example.com"}, req.Destination.BccAddresses)
		assert.Equal(t, []string{"reply@example.com"}, req.ReplyToAddresses)
		require.NotNil(t, req.ConfigurationSetName)
		assert.Equal(t, "my-set", *req.ConfigurationSetName)
		assert.Nil(t, req.Content.Simple.Body.Html)
	})
}

func TestValidateConfig(t *testing.T) {
	t.Run("valid config has no errors", func(t *testing.T) {
		errs := validateConfig(Config{
			AccessKeyID:     "AKIA...",
			SecretAccessKey: "secret",
			Region:          "us-east-1",
		})
		assert.Empty(t, errs)
	})

	t.Run("missing credentials are reported", func(t *testing.T) {
		errs := validateConfig(Config{})
		assert.Contains(t, errs, "accessKeyId")
		assert.Contains(t, errs, "secretAccessKey")
		assert.Contains(t, errs, "region")
	})
}

func TestClassifyError(t *testing.T) {
	assert.Equal(t, ExitSuccess, classifyError(nil))
	assert.Equal(t, ExitTransient, classifyError(errBody("Throttling: Rate exceeded")))
	assert.Equal(t, ExitTransient, classifyError(errBody("ses error (status 503): unavailable")))
	assert.Equal(t, ExitPermanent, classifyError(errBody("ses error (status 400): invalid recipient")))
}

func TestSignV4(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://email.us-east-1.amazonaws.com/v2/email/outbound-emails", strings.NewReader("{}"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	at := time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)
	SignV4(req, []byte("{}"), "AKIDEXAMPLE", "secret", "", "us-east-1", "ses", at)

	auth := req.Header.Get("Authorization")
	assert.True(t, strings.HasPrefix(auth, "AWS4-HMAC-SHA256 "), "unexpected auth header: %s", auth)
	assert.Contains(t, auth, "Credential=AKIDEXAMPLE/20260624/us-east-1/ses/aws4_request")
	assert.Contains(t, auth, "SignedHeaders=")
	assert.Contains(t, auth, "Signature=")
	assert.Equal(t, "20260624T120000Z", req.Header.Get("x-amz-date"))
	assert.NotEmpty(t, req.Header.Get("x-amz-content-sha256"))
}

// errBody is a small error implementation used to drive classifyError.
type errBody string

func (e errBody) Error() string { return string(e) }
