package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/extism/go-pdk"
	pdkhttp "github.com/extism/go-pdk/http"
	"github.com/lunogram/platform/pkg/modules"
	"github.com/lunogram/platform/pkg/modules/providers"
)

const (
	resendAPIURL     = "https://api.resend.com/emails"
	resendWebhookURL = "https://api.resend.com/webhooks"
)

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
			Webhook:  true,
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
									Name:   "webhookSecret",
									Schema: &modules.JSONSchema{Type: "string", Title: "Webhook Signing Secret", Format: "password", Description: "Svix webhook signing secret for verifying webhook signatures"},
									Hidden: true,
								},
								{
									Name:   "webhookId",
									Schema: &modules.JSONSchema{Type: "string", Title: "Webhook ID", Description: "Resend webhook ID (auto-configured)"},
									Hidden: true,
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
	APIKey        string `json:"apiKey"`
	WebhookSecret string `json:"webhookSecret"`
	WebhookID     string `json:"webhookId"`
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

// resendWebhookPayload is the JSON body from a Resend webhook callback.
type resendWebhookPayload struct {
	Type      string `json:"type"` // "email.delivered", "email.bounced", etc.
	CreatedAt string `json:"created_at"`
	Data      struct {
		EmailID string   `json:"email_id"`
		To      []string `json:"to"`
		From    string   `json:"from"`
		Subject string   `json:"subject"`
	} `json:"data"`
}

//go:export webhook
func WebhookHandler() int32 {
	var req providers.WebhookRequest
	if err := pdk.InputJSON(&req); err != nil {
		pdk.SetError(err)
		return exitPermanent
	}

	var config Config
	if err := json.Unmarshal(req.Config, &config); err != nil {
		pdk.SetError(fmt.Errorf("failed to parse config: %w", err))
		return exitPermanent
	}

	// Verify Svix webhook signature if a signing secret is configured.
	if config.WebhookSecret != "" {
		if err := verifySvixSignature(req.Headers, req.Body, config.WebhookSecret); err != nil {
			pdk.SetError(fmt.Errorf("invalid webhook signature: %w", err))
			return exitPermanent
		}
	}

	// Parse the Resend webhook payload.
	var payload resendWebhookPayload
	if err := json.Unmarshal(req.Body, &payload); err != nil {
		pdk.SetError(fmt.Errorf("failed to parse webhook body: %w", err))
		return exitPermanent
	}

	// Map Resend event type to canonical event name.
	var eventName providers.WebhookEventName
	switch payload.Type {
	case "email.delivered":
		eventName = providers.EventDelivered
	case "email.opened":
		eventName = providers.EventOpened
	case "email.clicked":
		eventName = providers.EventClicked
	case "email.bounced":
		eventName = providers.EventBounced
	case "email.complained":
		eventName = providers.EventComplained
	default:
		err := pdk.OutputJSON(providers.WebhookResponse{Events: []providers.WebhookEvent{}})
		if err != nil {
			pdk.SetError(err)
			return exitTransient
		}

		return 0
	}

	response := providers.WebhookResponse{
		Events: []providers.WebhookEvent{
			{
				EventName: eventName,
				MessageID: payload.Data.EmailID,
				Timestamp: payload.CreatedAt,
				Data: map[string]any{
					"to":      payload.Data.To,
					"from":    payload.Data.From,
					"subject": payload.Data.Subject,
					"type":    payload.Type,
				},
			},
		},
	}

	if err := pdk.OutputJSON(response); err != nil {
		pdk.SetError(err)
		return exitTransient
	}

	return 0
}

// verifySvixSignature verifies a Resend/Svix webhook signature.
//
// Svix signs webhooks using HMAC-SHA256. The signature is computed over
// "{msg_id}.{timestamp}.{body}" using a base64-decoded signing secret.
// The X-Svix-Signature header contains one or more base64-encoded signatures
// prefixed with "v1,".
//
// See: https://docs.svix.com/receiving/verifying-payloads/how
func verifySvixSignature(headers map[string]string, body json.RawMessage, secret string) error {
	msgID := headers["svix-id"]
	timestamp := headers["svix-timestamp"]
	signature := headers["svix-signature"]

	if msgID == "" || timestamp == "" || signature == "" {
		return fmt.Errorf("missing svix headers")
	}

	// Validate timestamp is within tolerance (5 minutes).
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid svix-timestamp: %w", err)
	}

	now := time.Now().Unix()
	diff := now - ts
	if diff < 0 {
		diff = -diff
	}
	if diff > 300 {
		return fmt.Errorf("svix-timestamp too old or in the future")
	}

	// Decode the signing secret. Svix secrets are prefixed with "whsec_".
	secretKey := strings.TrimPrefix(secret, "whsec_")

	keyBytes, err := base64.StdEncoding.DecodeString(secretKey)
	if err != nil {
		return fmt.Errorf("failed to decode webhook secret: %w", err)
	}

	// Compute expected signature: HMAC-SHA256("{msg_id}.{timestamp}.{body}").
	signedContent := msgID + "." + timestamp + "." + string(body)
	mac := hmac.New(sha256.New, keyBytes)
	mac.Write([]byte(signedContent))
	expectedSig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	// The header may contain multiple signatures separated by spaces.
	// Each signature is prefixed with a version (e.g., "v1,<base64>").
	for _, sig := range strings.Split(signature, " ") {
		parts := strings.SplitN(sig, ",", 2)
		if len(parts) != 2 {
			continue
		}
		if parts[0] == "v1" && hmac.Equal([]byte(parts[1]), []byte(expectedSig)) {
			return nil
		}
	}

	return fmt.Errorf("no matching v1 signature found")
}

//go:export validate
func Validate() int32 {
	var req providers.ValidateRequest
	if err := pdk.InputJSON(&req); err != nil {
		pdk.SetError(err)
		return exitPermanent
	}

	var config Config
	if err := json.Unmarshal(req.Config, &config); err != nil {
		pdk.SetError(fmt.Errorf("failed to parse config: %w", err))
		return exitPermanent
	}

	errs := make(map[string]string)

	if config.APIKey == "" {
		errs["apiKey"] = "API key is required"
	}

	if len(errs) > 0 {
		response := providers.ValidateResponse{
			Valid:   false,
			Errors:  errs,
			Message: "invalid provider configuration",
		}

		if err := pdk.OutputJSON(response); err != nil {
			pdk.SetError(err)
			return exitPermanent
		}
		return 0
	}

	if err := pdk.OutputJSON(providers.ValidateResponse{Valid: true}); err != nil {
		pdk.SetError(err)
		return exitPermanent
	}
	return 0
}

//go:export init
func Init() int32 {
	var req providers.InitRequest
	if err := pdk.InputJSON(&req); err != nil {
		pdk.SetError(err)
		return exitPermanent
	}

	var config Config
	if err := json.Unmarshal(req.Config, &config); err != nil {
		pdk.SetError(fmt.Errorf("failed to parse config: %w", err))
		return exitPermanent
	}

	if config.APIKey == "" {
		pdk.SetError(fmt.Errorf("missing required API key"))
		return exitPermanent
	}

	// Register a webhook with the Resend API.
	webhookBody, err := json.Marshal(map[string]any{
		"endpoint": req.WebhookURL,
		"events": []string{
			"email.sent",
			"email.delivered",
			"email.bounced",
			"email.opened",
			"email.clicked",
			"email.complained",
		},
	})
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to marshal webhook request: %w", err))
		return exitPermanent
	}

	httpReq, err := http.NewRequest(http.MethodPost, resendWebhookURL, bytes.NewReader(webhookBody))
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to create webhook request: %w", err))
		return exitPermanent
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+config.APIKey)

	httpClient := &http.Client{
		Transport: &safeTransport{inner: &pdkhttp.HTTPTransport{}},
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to register webhook with Resend: %w", err))
		return exitTransient
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to read webhook response: %w", err))
		return exitTransient
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		pdk.SetError(fmt.Errorf("Resend webhook registration failed (HTTP %d): %s", resp.StatusCode, string(respBody)))
		return exitTransient
	}

	// Parse the response to get the webhook ID and signing secret.
	var result struct {
		ID            string `json:"id"`
		SigningSecret string `json:"signing_secret"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		pdk.SetError(fmt.Errorf("failed to parse webhook response: %w", err))
		return exitTransient
	}

	// Return a config patch so the platform persists the webhook ID and secret.
	patch, err := json.Marshal(map[string]string{
		"webhookSecret": result.SigningSecret,
		"webhookId":     result.ID,
	})
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to marshal config patch: %w", err))
		return exitTransient
	}

	if err := pdk.OutputJSON(providers.InitResponse{
		ConfigPatch: patch,
	}); err != nil {
		pdk.SetError(err)
		return exitTransient
	}

	return 0
}

//go:export destroy
func Destroy() int32 {
	var req providers.DestroyRequest
	if err := pdk.InputJSON(&req); err != nil {
		pdk.SetError(err)
		return exitPermanent
	}

	var config Config
	if err := json.Unmarshal(req.Config, &config); err != nil {
		pdk.SetError(fmt.Errorf("failed to parse config: %w", err))
		return exitPermanent
	}

	// Nothing to clean up if no webhook was registered.
	if config.WebhookID == "" {
		if err := pdk.OutputJSON(providers.DestroyResponse{}); err != nil {
			pdk.SetError(err)
			return exitTransient
		}
		return 0
	}

	endpoint := resendWebhookURL + "/" + config.WebhookID
	httpReq, err := http.NewRequest(http.MethodDelete, endpoint, nil)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to create delete request: %w", err))
		return exitPermanent
	}

	httpReq.Header.Set("Authorization", "Bearer "+config.APIKey)

	httpClient := &http.Client{
		Transport: &safeTransport{inner: &pdkhttp.HTTPTransport{}},
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to delete webhook from Resend: %w", err))
		return exitTransient
	}
	defer resp.Body.Close()

	// 404 is acceptable — the webhook may have been deleted externally.
	if resp.StatusCode >= 300 && resp.StatusCode != 404 {
		respBody, _ := io.ReadAll(resp.Body)
		pdk.SetError(fmt.Errorf("Resend webhook deletion failed (HTTP %d): %s", resp.StatusCode, string(respBody)))
		return exitTransient
	}

	if err := pdk.OutputJSON(providers.DestroyResponse{}); err != nil {
		pdk.SetError(err)
		return exitTransient
	}

	return 0
}

func main() {}
