package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

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
								{
									Name:   "default_from",
									Schema: &modules.JSONSchema{Type: "string", Title: "Default From Address", Description: "Default sender email address"},
								},
								{
									Name:   "default_from_name",
									Schema: &modules.JSONSchema{Type: "string", Title: "Default From Name", Description: "Default sender display name"},
								},
								{
									Name:   "default_from_locked",
									Schema: &modules.JSONSchema{Type: "boolean", Title: "Lock From Address", Description: "Prevent templates from overriding the from address"},
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
		return -1
	}

	// Only email channel is supported
	if req.Channel != providers.ChannelEmail {
		pdk.SetError(fmt.Errorf("unsupported channel: %s", req.Channel))
		return -1
	}

	// Get email payload
	email, err := req.GetEmailPayload()
	if err != nil {
		pdk.SetError(err)
		return -1
	}

	// Validate required fields
	if email.From.Address == "" {
		pdk.SetError(fmt.Errorf("missing required 'from' address"))
		return -1
	}

	if email.Subject == "" {
		pdk.SetError(fmt.Errorf("missing required 'subject'"))
		return -1
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
		return -1
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

func main() {}
