package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/extism/go-pdk"
	pdkhttp "github.com/extism/go-pdk/http"
	"github.com/lunogram/platform/pkg/modules"
	"github.com/lunogram/platform/pkg/modules/providers"
)

const resendAPIURL = "https://api.resend.com/emails"

// safeTransport wraps the Extism HTTPTransport to guarantee that resp.Body is
// never nil. The standard http.Client contract promises a non-nil Body, but the
// Extism PDK transport can return nil when the response has no content.
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

// resendEmailRequest is the JSON body for POST https://api.resend.com/emails.
// Only the fields we use are included — no interface{} types that would trigger
// TinyGo reflectlite panics.
type resendEmailRequest struct {
	From    string            `json:"from"`
	To      []string          `json:"to"`
	Subject string            `json:"subject"`
	HTML    string            `json:"html,omitempty"`
	Text    string            `json:"text,omitempty"`
	Cc      []string          `json:"cc,omitempty"`
	Bcc     []string          `json:"bcc,omitempty"`
	ReplyTo string            `json:"reply_to,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// resendEmailResponse is the success response from the Resend API.
type resendEmailResponse struct {
	ID string `json:"id"`
}

// resendErrorResponse is the error response from the Resend API.
type resendErrorResponse struct {
	StatusCode int    `json:"statusCode"`
	Name       string `json:"name"`
	Message    string `json:"message"`
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

	// Build Resend API request body
	body := resendEmailRequest{
		From:    formatAddress(email.From),
		To:      []string{email.To},
		Subject: email.Subject,
		HTML:    email.HTML,
		Text:    email.Text,
		Headers: email.Headers,
	}

	if email.Cc != nil {
		body.Cc = []string{*email.Cc}
	}

	if email.Bcc != nil {
		body.Bcc = []string{*email.Bcc}
	}

	if email.ReplyTo != nil {
		body.ReplyTo = *email.ReplyTo
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to marshal request: %w", err))
		return exitPermanent
	}

	// Create HTTP request
	httpReq, err := http.NewRequest(http.MethodPost, resendAPIURL, bytes.NewReader(jsonBody))
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to create request: %w", err))
		return exitPermanent
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+req.Config.APIKey)

	// Send via Extism PDK transport
	httpClient := &http.Client{
		Transport: &safeTransport{inner: &pdkhttp.HTTPTransport{}},
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to send email: %w", err))
		return exitTransient
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to read response: %w", err))
		return exitTransient
	}

	// Handle error responses
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return handleErrorResponse(resp.StatusCode, respBody)
	}

	// Parse success response
	var resendResp resendEmailResponse
	if err := json.Unmarshal(respBody, &resendResp); err != nil {
		pdk.SetError(fmt.Errorf("failed to parse response: %w", err))
		return exitTransient
	}

	response := providers.SendResponse{
		ID:     resendResp.ID,
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

// handleErrorResponse maps a Resend API error response to an exit code.
//
// Classification:
//   - 429 (rate limit)              → transient (retry later)
//   - 500+ (server error)           → transient (retry later)
//   - 400, 401, 403, 422 (client)   → permanent (will never succeed)
func handleErrorResponse(statusCode int, body []byte) int32 {
	var errResp resendErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		// Can't parse error body — use status code only.
		msg := fmt.Sprintf("resend API error (HTTP %d): %s", statusCode, string(body))
		pdk.SetError(fmt.Errorf("%s", msg))
	} else {
		pdk.SetError(fmt.Errorf("resend API error (HTTP %d): %s: %s", statusCode, errResp.Name, errResp.Message))
	}

	switch {
	case statusCode == 429:
		return exitTransient
	case statusCode >= 500:
		return exitTransient
	case strings.HasPrefix(fmt.Sprintf("%d", statusCode), "4"):
		return exitPermanent
	default:
		return exitTransient
	}
}

func main() {}
