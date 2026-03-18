package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/extism/go-pdk"
	pdkhttp "github.com/extism/go-pdk/http"
	"github.com/lunogram/platform/pkg/modules"
	"github.com/lunogram/platform/pkg/modules/providers"
	"github.com/resend/resend-go/v3"
)

// NewResendClient creates a Resend SDK client that routes HTTP through the
// Extism PDK transport so it works inside WASM.
func NewResendClient(apiKey string) *resend.Client {
	httpClient := &http.Client{
		Transport: &safeTransport{inner: &pdkhttp.HTTPTransport{}},
	}
	return resend.NewCustomClient(httpClient, apiKey)
}

// safeTransport wraps an http.RoundTripper to guarantee that resp.Body is
// never nil. The standard http.Client contract promises a non-nil Body, but
// the Extism PDK transport can return nil when the response has no content.
type safeTransport struct {
	inner http.RoundTripper
}

func (t *safeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp != nil && resp.Body == nil {
		resp.Body = http.NoBody
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

	if err := pdk.OutputJSON(manifest); err != nil {
		pdk.SetError(err)
		return -1
	}
	return ExitSuccess
}

//go:export send
func Send() int32 {
	var req providers.SendRequest[Config]
	if err := pdk.InputJSON(&req); err != nil {
		pdk.SetError(err)
		return ExitPermanent
	}

	if req.Channel != providers.ChannelEmail {
		pdk.SetError(fmt.Errorf("unsupported channel: %s", req.Channel))
		return ExitPermanent
	}

	email, err := req.GetEmailPayload()
	if err != nil {
		pdk.SetError(err)
		return ExitPermanent
	}

	if email.From.Address == "" {
		pdk.SetError(fmt.Errorf("missing required 'from' address"))
		return ExitPermanent
	}
	if email.Subject == "" {
		pdk.SetError(fmt.Errorf("missing required 'subject'"))
		return ExitPermanent
	}
	if email.HTML == "" && email.Text == "" {
		pdk.SetError(fmt.Errorf("missing required 'html' or 'text' body content"))
		return ExitPermanent
	}

	client := NewResendClient(req.Config.APIKey)
	sendReq := ComposeSendEmailRequest(email)

	// Log the outgoing request payload for debugging 422 errors from Resend.
	debugPayload, _ := json.Marshal(struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		HasHTML bool     `json:"has_html"`
		HasText bool     `json:"has_text"`
	}{
		From:    sendReq.From,
		To:      sendReq.To,
		Subject: sendReq.Subject,
		HasHTML: sendReq.Html != "",
		HasText: sendReq.Text != "",
	})
	pdk.Log(pdk.LogDebug, fmt.Sprintf("resend send request: %s", string(debugPayload)))

	resp, err := client.Emails.Send(sendReq)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to send email (from=%s, to=%v, subject=%q): %w", sendReq.From, sendReq.To, sendReq.Subject, err))
		return classifyError(err)
	}

	err = pdk.OutputJSON(providers.SendResponse{
		ID:     resp.Id,
		Status: "sent",
	})
	if err != nil {
		pdk.SetError(err)
		return -1
	}
	return ExitSuccess
}

//go:export webhook
func WebhookHandler() int32 {
	var req providers.WebhookRequest
	if err := pdk.InputJSON(&req); err != nil {
		pdk.SetError(err)
		return ExitPermanent
	}

	var config Config
	if err := json.Unmarshal(req.Config, &config); err != nil {
		pdk.SetError(fmt.Errorf("failed to parse config: %w", err))
		return ExitPermanent
	}

	// Verify Svix webhook signature using the SDK if a signing secret is configured.
	if config.WebhookSecret != "" {
		client := NewResendClient(config.APIKey)
		err := client.Webhooks.Verify(&resend.VerifyWebhookOptions{
			Payload: string(req.Body),
			Headers: resend.WebhookHeaders{
				Id:        req.Headers["svix-id"],
				Timestamp: req.Headers["svix-timestamp"],
				Signature: req.Headers["svix-signature"],
			},
			WebhookSecret: config.WebhookSecret,
		})
		if err != nil {
			pdk.SetError(fmt.Errorf("invalid webhook signature: %w", err))
			return ExitPermanent
		}
	}

	// Parse the Resend webhook payload.
	var payload resendWebhookPayload
	if err := json.Unmarshal(req.Body, &payload); err != nil {
		pdk.SetError(fmt.Errorf("failed to parse webhook body: %w", err))
		return ExitPermanent
	}

	// Map Resend event type to canonical event name.
	eventName, ok := mapWebhookEvent(payload.Type)
	if !ok {
		// Unrecognised event — return empty events list (not an error).
		err := pdk.OutputJSON(providers.WebhookResponse{Events: []providers.WebhookEvent{}})
		if err != nil {
			pdk.SetError(err)
			return ExitTransient
		}

		return ExitSuccess
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
		return ExitTransient
	}
	return ExitSuccess
}

//go:export validate
func Validate() int32 {
	var req providers.ValidateRequest
	if err := pdk.InputJSON(&req); err != nil {
		pdk.SetError(err)
		return ExitPermanent
	}

	var config Config
	if err := json.Unmarshal(req.Config, &config); err != nil {
		pdk.SetError(fmt.Errorf("failed to parse config: %w", err))
		return ExitPermanent
	}

	errs := make(map[string]string)
	if config.APIKey == "" {
		errs["apiKey"] = "API key is required"
	}

	if len(errs) > 0 {
		if err := pdk.OutputJSON(providers.ValidateResponse{
			Valid:   false,
			Errors:  errs,
			Message: "invalid provider configuration",
		}); err != nil {
			pdk.SetError(err)
			return ExitPermanent
		}
		return ExitSuccess
	}

	if err := pdk.OutputJSON(providers.ValidateResponse{Valid: true}); err != nil {
		pdk.SetError(err)
		return ExitPermanent
	}
	return ExitSuccess
}

//go:export init
func Init() int32 {
	var req providers.InitRequest
	if err := pdk.InputJSON(&req); err != nil {
		pdk.SetError(err)
		return ExitPermanent
	}

	var config Config
	if err := json.Unmarshal(req.Config, &config); err != nil {
		pdk.SetError(fmt.Errorf("failed to parse config: %w", err))
		return ExitPermanent
	}

	if config.APIKey == "" {
		pdk.SetError(fmt.Errorf("missing required API key"))
		return ExitPermanent
	}

	client := NewResendClient(config.APIKey)
	res, err := client.Webhooks.Create(&resend.CreateWebhookRequest{
		Endpoint: req.WebhookURL,
		Events:   webhookEvents(),
	})
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to register webhook with Resend: %w", err))
		return ExitTransient
	}

	// Return a config patch so the platform persists the webhook ID and signing secret.
	patch, err := json.Marshal(map[string]string{
		"webhookSecret": res.SigningSecret,
		"webhookId":     res.Id,
	})
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to marshal config patch: %w", err))
		return ExitTransient
	}

	err = pdk.OutputJSON(providers.InitResponse{
		ConfigPatch: patch,
	})
	if err != nil {
		pdk.SetError(err)
		return ExitTransient
	}
	return ExitSuccess
}

//go:export destroy
func Destroy() int32 {
	var req providers.DestroyRequest
	if err := pdk.InputJSON(&req); err != nil {
		pdk.SetError(err)
		return ExitPermanent
	}

	var config Config
	if err := json.Unmarshal(req.Config, &config); err != nil {
		pdk.SetError(fmt.Errorf("failed to parse config: %w", err))
		return ExitPermanent
	}

	// Nothing to clean up if no webhook was registered.
	if config.WebhookID == "" {
		if err := pdk.OutputJSON(providers.DestroyResponse{}); err != nil {
			pdk.SetError(err)
			return ExitTransient
		}
		return ExitSuccess
	}

	client := NewResendClient(config.APIKey)
	_, err := client.Webhooks.Remove(config.WebhookID)
	if err != nil {
		// If the webhook was already deleted externally, that's fine.
		if !strings.Contains(err.Error(), "not_found") && !strings.Contains(err.Error(), "404") {
			pdk.SetError(fmt.Errorf("failed to delete webhook from Resend: %w", err))
			return ExitTransient
		}
	}

	err = pdk.OutputJSON(providers.DestroyResponse{})
	if err != nil {
		pdk.SetError(err)
		return ExitTransient
	}
	return ExitSuccess
}

func main() {}
