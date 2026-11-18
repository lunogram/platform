//go:build tinygo

// Exports use //go:wasmexport rather than //go:export: TinyGo runs a
// //go:export function without a current goroutine, and crypto/sha256 then
// panics reading its per-goroutine FIPS indicator, which would break webhook
// signature verification. The Extism imports these exports pull in exist only
// in the host, so this file is built by TinyGo alone and tests run with plain
// go test.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/extism/go-pdk"
	pdkhttp "github.com/extism/go-pdk/http"
	"github.com/lunogram/platform/pkg/modules"
	"github.com/lunogram/platform/pkg/modules/providers"
)

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

// callAhaSend posts a JSON body to AhaSend. A non-empty idempotencyKey makes
// AhaSend replay the stored result when a redelivered send repeats the request.
func callAhaSend(ctx context.Context, apiKey, endpoint, idempotencyKey string, body []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := newHTTPClient().Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}

	return resp.StatusCode, respBody, nil
}

// providerCapabilitySpec describes the provider capability advertised by this
// integration.
func providerCapabilitySpec() json.RawMessage {
	spec, err := json.Marshal(modules.ProviderSpec{
		Channels: []modules.Channel{modules.ChannelEmail},
		Webhook:  true,
		RateLimit: &modules.RateLimit{
			Limit:    5,
			Interval: "1s",
			Override: true,
		},
	})
	if err != nil {
		panic(err)
	}

	return spec
}

//go:wasmexport manifest
func Manifest() int32 {
	manifest := modules.IntegrationManifest{
		APIVersion: "v1",
		Metadata: modules.Metadata{
			ID:          "ahasend",
			Title:       "AhaSend Email",
			Description: "AhaSend email service integration",
			Icon:        "https://ahasend.com/favicon.svg",
			Color:       "#414042",
			Tags:        []string{"email"},
		},
		Website: "https://ahasend.com",
		Version: "1.0.0",
		License: "MIT",
		Author: modules.Author{
			Name:  "Lunogram",
			Email: "dev@lunogram.io",
			URL:   "https://lunogram.com",
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
								Schema: &modules.JSONSchema{Type: "string", Title: "AhaSend API Key", Format: "password", Description: "API key v2 with the messages:send scope for your sending domain"},
							},
							{
								Name:   "accountId",
								Schema: &modules.JSONSchema{Type: "string", Title: "Account ID", Description: "AhaSend account ID, shown in the dashboard's account settings"},
							},
							{
								Name:   "webhookSecret",
								Schema: &modules.JSONSchema{Type: "string", Title: "Webhook Secret", Format: "password", Description: "Optional AhaSend webhook secret (aha-whsec-…) used to verify webhook signatures"},
							},
						},
						Required: []string{"apiKey", "accountId"},
					},
				},
			},
		},
		Capabilities: []modules.Capability{
			{
				Type:    "provider",
				Version: "v1",
				Spec:    providerCapabilitySpec(),
			},
		},
	}

	if err := pdk.OutputJSON(manifest); err != nil {
		pdk.SetError(err)
		return ExitTransient
	}

	return ExitSuccess
}

//go:wasmexport provider_send
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

	sendReq, err := ComposeSendEmailRequest(email)
	if err != nil {
		pdk.SetError(err)
		return ExitPermanent
	}

	body, err := json.Marshal(sendReq)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to encode AhaSend request: %w", err))
		return ExitPermanent
	}

	endpoint := fmt.Sprintf("%s/v2/accounts/%s/messages/conversation", ahaSendAPIBase, url.PathEscape(req.Config.AccountID))
	idempotencyKey := req.Metadata[providers.MetadataKeyInboxMessageID]
	status, respBody, err := callAhaSend(context.Background(), req.Config.APIKey, endpoint, idempotencyKey, body)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to call AhaSend API: %w", err))
		return ExitTransient
	}

	if status < 200 || status >= 300 {
		pdk.SetError(fmt.Errorf("ahasend API error (status=%d): %s", status, string(respBody)))
		return classifyHTTPStatus(status)
	}

	var sendResp ahaSendMessageResponse
	if err := json.Unmarshal(respBody, &sendResp); err != nil {
		pdk.SetError(fmt.Errorf("failed to decode AhaSend send response: %w", err))
		return ExitTransient
	}

	id, err := messageID(sendResp, email.To)
	if err != nil {
		pdk.SetError(err)
		return ExitPermanent
	}

	if err := pdk.OutputJSON(providers.SendResponse{ID: id, Status: "sent"}); err != nil {
		pdk.SetError(err)
		return ExitTransient
	}

	return ExitSuccess
}

//go:wasmexport provider_webhook
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

	if config.WebhookSecret != "" {
		if err := verifyAhaSendWebhookSignature(config.WebhookSecret, req.Headers, req.Body); err != nil {
			pdk.SetError(fmt.Errorf("failed to verify ahasend webhook signature: %w", err))
			return ExitPermanent
		}
	}

	event, ok, err := parseAhaSendWebhookEvent(req.Body)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to parse ahasend webhook body: %w", err))
		return ExitPermanent
	}

	events := []providers.WebhookEvent{}
	if ok {
		events = append(events, event)
	}

	if err := pdk.OutputJSON(providers.WebhookResponse{Events: events}); err != nil {
		pdk.SetError(err)
		return ExitTransient
	}

	return ExitSuccess
}

//go:wasmexport validate
func Validate() int32 {
	var req modules.ValidateRequest
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
		if err := pdk.OutputJSON(modules.ValidateResponse{
			Valid:   false,
			Errors:  errs,
			Message: "invalid provider configuration",
		}); err != nil {
			pdk.SetError(err)
			return ExitPermanent
		}
		return ExitSuccess
	}

	if err := pdk.OutputJSON(modules.ValidateResponse{Valid: true}); err != nil {
		pdk.SetError(err)
		return ExitPermanent
	}

	return ExitSuccess
}
