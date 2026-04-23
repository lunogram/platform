package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/extism/go-pdk"
	pdkhttp "github.com/extism/go-pdk/http"
	"github.com/lunogram/platform/pkg/modules"
	"github.com/lunogram/platform/pkg/modules/providers"
)

const sendGridSendURL = "https://api.sendgrid.com/v3/mail/send"

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

func newHTTPClient() *http.Client {
	return &http.Client{Transport: &safeTransport{inner: &pdkhttp.HTTPTransport{}}}
}

//go:export manifest
func Manifest() int32 {
	manifest := providers.ProviderManifest{
		Metadata: modules.Metadata{
			ID:          "sendgrid",
			Title:       "SendGrid Email",
			Description: "SendGrid email service integration",
			Icon:        "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSI2NCIgaGVpZ2h0PSI2NCI+PHBhdGggZD0iTTAgMjEuMjVoMjEuMzc0djIxLjM3NEgweiIgZmlsbD0iI2ZmZiIvPjxwYXRoIGQ9Ik0wIDIxLjI1aDIxLjM3NHYyMS4zNzRIMHoiIGZpbGw9IiM5OWUxZjQiIGVuYWJsZS1iYWNrZ3JvdW5kPSJuZXcgIi8+PHBhdGggZD0iTTIxLjM3NCA0Mi42MjZoMjEuMjV2MjEuMjVoLTIxLjI1eiIgZmlsbD0iI2ZmZiIvPjxwYXRoIGQ9Ik0yMS4zNzQgNDIuNjI2aDIxLjI1djIxLjI1aC0yMS4yNXoiIGZpbGw9IiM5OWUxZjQiIGVuYWJsZS1iYWNrZ3JvdW5kPSJuZXcgIi8+PHBhdGggZD0iTTAgNjMuODc3aDIxLjM3NFY2NEgwem0wLTIxLjI1aDIxLjM3NHYyMS4yNUgweiIgZmlsbD0iIzFhODJlMiIvPjxwYXRoIGQ9Ik0yMS4zNzQgMGgyMS4yNXYyMS4yNWgtMjEuMjV6bTIxLjI1MiAyMS4zNzRINjR2MjEuMjVINDIuNjI2eiIgZmlsbD0iIzAwYjNlMyIvPjxwYXRoIGQ9Ik0yMS4zNzQgNDIuNjI2aDIxLjI1VjIxLjI1aC0yMS4yNXoiIGZpbGw9IiMwMDlkZDkiLz48ZyBmaWxsPSIjMWE4MmUyIj48cGF0aCBkPSJNNDIuNjI2IDBINjR2MjEuMjVINDIuNjI2eiIvPjxwYXRoIGQ9Ik00Mi42MjYgMjEuMjVINjR2LjEyM0g0Mi42MjZ6Ii8+PC9nPjwvc3ZnPg==",
			Color:       "#0D74FF",
			Tags:        []string{"email"},
		},
		Website: "https://sendgrid.com",
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
			RateLimit: &providers.RateLimit{
				Limit:    5,
				Interval: "1s",
				Override: true,
			},
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
									Schema: &modules.JSONSchema{Type: "string", Title: "SendGrid API Key", Format: "password"},
								},
								{
									Name:   "webhookVerificationKey",
									Schema: &modules.JSONSchema{Type: "string", Title: "Webhook Verification Key", Description: "SendGrid signed webhook public key in PEM format (optional)"},
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

	if email.To == "" {
		pdk.SetError(fmt.Errorf("missing required 'to' address"))
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

	body, err := json.Marshal(ComposeSendEmailRequest(email))
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to marshal sendgrid payload: %w", err))
		return ExitPermanent
	}

	httpReq, err := http.NewRequest(http.MethodPost, sendGridSendURL, bytes.NewReader(body))
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to create request: %w", err))
		return ExitTransient
	}
	httpReq.Header.Set("Authorization", "Bearer "+req.Config.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := newHTTPClient().Do(httpReq)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to call sendgrid API: %w", err))
		return ExitTransient
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to read sendgrid response: %w", err))
		return ExitTransient
	}

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		errMsg := string(respBody)
		var apiErr sendGridErrorBody
		if json.Unmarshal(respBody, &apiErr) == nil {
			if formatted := formatSendGridErrors(apiErr); formatted != "" {
				errMsg = formatted
			}
		}

		pdk.SetError(fmt.Errorf("sendgrid API error (status=%d): %s", httpResp.StatusCode, errMsg))
		return classifyHTTPStatus(httpResp.StatusCode)
	}

	messageID := httpResp.Header.Get("X-Message-Id")
	if err := pdk.OutputJSON(providers.SendResponse{ID: messageID, Status: "sent"}); err != nil {
		pdk.SetError(err)
		return ExitTransient
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

	if config.WebhookVerificationKey != "" {
		signature := req.Headers["x-twilio-email-event-webhook-signature"]
		timestamp := req.Headers["x-twilio-email-event-webhook-timestamp"]
		if err := verifySendGridWebhookSignature(config.WebhookVerificationKey, req.Body, signature, timestamp); err != nil {
			pdk.SetError(fmt.Errorf("failed to verify sendgrid webhook signature: %w", err))
			return ExitPermanent
		}
	}

	events, err := parseSendGridWebhookEvents(req.Body)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to parse sendgrid webhook body: %w", err))
		return ExitPermanent
	}

	if err := pdk.OutputJSON(providers.WebhookResponse{Events: events}); err != nil {
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

	errs := validateConfig(config)

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

func main() {}
