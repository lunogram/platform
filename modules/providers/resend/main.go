package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/extism/go-pdk"
	pdkhttp "github.com/extism/go-pdk/http"
	"github.com/lunogram/platform/pkg/modules"
	"github.com/lunogram/platform/pkg/modules/providers"
	"github.com/resend/resend-go/v3"
)

// safeTransport wraps the Extism HTTPTransport to guarantee that resp.Body is
// never nil. The standard http.Client contract promises a non-nil Body, but the
// Extism PDK transport can return nil when the response has no content. Third-
// party libraries like the Resend SDK call resp.Body.Close() unconditionally,
// which causes a nil-panic in WASM without this wrapper.
type safeTransport struct {
	inner http.RoundTripper
}

func (t *safeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp.Body == nil {
		resp.Body = io.NopCloser(bytes.NewReader(nil))
	}
	return resp, nil
}

//go:export manifest
func Manifest() int32 {
	manifest := providers.ProviderManifest{
		Metadata: modules.Metadata{
			ID:          "resend",
			Title:       "Resend Email",
			Description: "Resend email service integration",
			Icon:        "https://cdn.resend.com/brand/resend-icon-black.svg",
			Color:       "#000000",
			Tags:        []string{"email"},
		},
		Website: "https://resend.com",
		Version: "1.0.0",
		License: "MIT",
		Author: modules.Author{
			Name:  "Lunogram",
			Email: "dev@lunogram.io",
			URL:   "https://lunogram.com",
		},
		Spec: providers.ProviderSpec{
			Channels: []providers.Channel{providers.ChannelEmail},
			Config: &modules.JSONSchema{
				Type: "object",
				Properties: []modules.JSONSchemaProperty{
					{
						Name: "data",
						Schema: &modules.JSONSchema{
							Type: "object",
							Properties: []modules.JSONSchemaProperty{
								{
									Name:   "apiKey",
									Schema: &modules.JSONSchema{Type: "string", Title: "Resend API Key", Format: "password"},
								},
							},
							Required: []string{"apiKey"},
						},
					},
				},
			},
		},
	}

	err := pdk.OutputJSON(manifest)
	if err != nil {
		pdk.SetError(err)
		return -1
	}

	return 0
}

type Config struct {
	APIKey string `json:"apiKey"`
}

//go:export send
func Send() int32 {
	var req providers.SendRequest[Config]
	err := pdk.InputJSON(&req)
	if err != nil {
		pdk.SetError(err)
		return exitPermanent
	}

	// Only email channel is supported
	if req.Channel != providers.ChannelEmail {
		pdk.SetError(fmt.Errorf("unsupported channel: %s", req.Channel))
		return exitPermanent
	}

	// Get email payload
	email, err := req.GetEmailPayload()
	if err != nil {
		pdk.SetError(err)
		return exitPermanent
	}

	// Validate required fields
	if email.From.Address == "" {
		pdk.SetError(fmt.Errorf("missing required 'from' address"))
		return exitPermanent
	}

	if email.Subject == "" {
		pdk.SetError(fmt.Errorf("missing required 'subject'"))
		return exitPermanent
	}

	// Create HTTP client for WASM
	httpClient := &http.Client{
		Transport: &safeTransport{inner: &pdkhttp.HTTPTransport{}},
	}

	client := resend.NewCustomClient(httpClient, req.Config.APIKey)

	params := &resend.SendEmailRequest{
		From:    formatAddress(email.From),
		To:      []string{email.To},
		Html:    email.HTML,
		Subject: email.Subject,
		Headers: email.Headers,
	}

	if email.Cc != nil {
		params.Cc = []string{*email.Cc}
	}

	if email.Bcc != nil {
		params.Bcc = []string{*email.Bcc}
	}

	if email.ReplyTo != nil {
		params.ReplyTo = *email.ReplyTo
	}

	if email.Text != "" {
		params.Text = email.Text
	}

	sent, err := client.Emails.Send(params)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to send email: %w", err))
		return classifyError(err)
	}

	response := providers.SendResponse{
		ID:     sent.Id,
		Status: "sent",
	}

	err = pdk.OutputJSON(response)
	if err != nil {
		pdk.SetError(err)
		return -1
	}

	return 0
}

func formatAddress(address providers.EmailAddress) string {
	if address.Name != "" {
		return fmt.Sprintf("%s <%s>", address.Name, address.Address)
	}
	return address.Address
}

// Exit code convention for WASM provider modules:
//
//	 0  — success
//	-1  — transient/retryable error  (rate limit, network, server error)
//	-2  — permanent/non-retryable error (invalid recipient, validation, auth)
const (
	exitTransient int32 = -1
	exitPermanent int32 = -2
)

// classifyError maps a Resend SDK error to an exit code.
//
// The Resend Go SDK (v3) exposes only *RateLimitError as a typed error;
// all other API errors (400, 401, 403, 422, 500, etc.) are returned as
// plain errors with message format "[ERROR]: <message>".
//
// Classification:
//   - *RateLimitError (429)         → transient (retry later)
//   - "[ERROR]: " validation msgs  → permanent (will never succeed)
//   - Everything else (network, unknown) → transient (safe default)
func classifyError(err error) int32 {
	// Rate limit → always transient.
	var rateLimitErr *resend.RateLimitError
	if errors.As(err, &rateLimitErr) {
		return exitTransient
	}

	msg := err.Error()

	// Resend API 400/422 errors follow the "[ERROR]: " prefix pattern.
	// These are validation failures that will never succeed on retry.
	if strings.Contains(msg, "[ERROR]: ") {
		return exitPermanent
	}

	// Default to transient for unknown/network errors.
	return exitTransient
}

func main() {}
