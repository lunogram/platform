package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/extism/go-pdk"
	"github.com/lunogram/platform/modules/providers/twilio/provider"
	"github.com/lunogram/platform/pkg/modules"
	"github.com/lunogram/platform/pkg/modules/providers"
)

func providerCapabilitySpec() json.RawMessage {
	spec, err := json.Marshal(modules.ProviderSpec{
		Channels: []modules.Channel{modules.ChannelSMS},
		Webhook:  true,
		RateLimit: &modules.RateLimit{
			Limit:    1,
			Interval: "1s",
			Override: false,
		},
	})
	if err != nil {
		panic(err)
	}

	return spec
}

//go:export manifest
func Manifest() int32 {
	manifest := modules.IntegrationManifest{
		APIVersion: "v1",
		Metadata: modules.Metadata{
			ID:          "twilio",
			Title:       "Twilio SMS",
			Description: "Send SMS messages via Twilio with delivery tracking",
			Icon:        "https://static.cdnlogo.com/logos/t/14/twilio.svg",
			Color:       "#c7252b",
			Tags:        []string{"sms"},
		},
		Website: "https://twilio.com",
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
								Name:   "accountSid",
								Schema: &modules.JSONSchema{Type: "string", Title: "Account SID"},
							},
							{
								Name:   "authToken",
								Schema: &modules.JSONSchema{Type: "string", Title: "Auth Token", Format: "password"},
							},
							{
								Name:   "webhookUrl",
								Schema: &modules.JSONSchema{Type: "string", Title: "Webhook URL", Description: "Platform webhook callback URL (auto-configured)"},
								Hidden: true,
							},
						},
						Required: []string{"accountSid", "authToken"},
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
		return -1
	}
	return ExitSuccess
}

//go:export provider_send
func Send() int32 {
	var req providers.SendRequest[Config]
	if err := pdk.InputJSON(&req); err != nil {
		pdk.SetError(err)
		return ExitPermanent
	}

	if req.Channel != providers.ChannelSMS {
		pdk.SetError(fmt.Errorf("unsupported channel: %s (twilio provider supports sms only)", req.Channel))
		return ExitPermanent
	}

	sms, err := req.GetSMSPayload()
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to parse SMS payload: %w", err))
		return ExitPermanent
	}

	if sms.To == "" {
		pdk.SetError(fmt.Errorf("missing required 'to' phone number"))
		return ExitPermanent
	}
	if sms.Body == "" {
		pdk.SetError(fmt.Errorf("missing required 'body' text"))
		return ExitPermanent
	}

	from, ok := provider.ResolveSender(sms.From)
	if !ok {
		pdk.SetError(fmt.Errorf("missing sender phone number: configure a sender identity for this provider"))
		return ExitPermanent
	}

	client := NewTwilioClient(req.Config.AccountSID, req.Config.AuthToken)

	params := &CreateMessageParams{}
	params.SetTo(sms.To)
	params.SetFrom(from)
	params.SetBody(sms.Body)

	if req.Config.WebhookURL != "" {
		params.SetStatusCallback(req.Config.WebhookURL)
	}

	if len(sms.MediaURLs) > 0 {
		params.SetMediaUrl(sms.MediaURLs)
	}

	pdk.Log(pdk.LogInfo, fmt.Sprintf("sending SMS via Twilio to=%s from=%s", sms.To, from))

	result := client.CreateMessage(params)
	if result.Err != nil {
		pdk.Log(pdk.LogError, fmt.Sprintf("Twilio API error (http_status=%d): %s", result.HTTPStatus, result.Err))
		pdk.SetError(fmt.Errorf("failed to send SMS (to=%s, from=%s): %w", sms.To, from, result.Err))
		if result.HTTPStatus > 0 {
			return provider.ClassifyHTTPStatus(result.HTTPStatus)
		}
		return ExitTransient
	}

	messageID := ""
	if result.Response.Sid != nil {
		messageID = *result.Response.Sid
	}

	status := "queued"
	if result.Response.Status != nil {
		status = *result.Response.Status
	}

	pdk.Log(pdk.LogInfo, fmt.Sprintf("Twilio API response: sid=%s status=%s http_status=%d", messageID, status, result.HTTPStatus))

	if err := pdk.OutputJSON(providers.SendResponse{
		ID:     messageID,
		Status: status,
		Metadata: map[string]any{
			"channel": string(providers.ChannelSMS),
		},
	}); err != nil {
		pdk.SetError(err)
		return ExitTransient
	}
	return ExitSuccess
}

//go:export provider_webhook
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

	signature := req.Headers["x-twilio-signature"]
	if signature == "" {
		pdk.SetError(fmt.Errorf("missing x-twilio-signature header"))
		return ExitPermanent
	}

	params, err := provider.ParseWebhookParams(req.Body)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to parse webhook body as form params: %w", err))
		return ExitPermanent
	}

	validator := NewRequestValidator(config.AuthToken)
	if !validator.Validate(req.URL, params, signature) {
		pdk.SetError(fmt.Errorf("invalid Twilio webhook signature"))
		return ExitPermanent
	}

	payload, err := provider.ParseWebhookBody(req.Body)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to parse webhook body: %w", err))
		return ExitPermanent
	}

	eventName, ok := provider.MapWebhookStatus(payload.MessageStatus)
	if !ok {
		if err := pdk.OutputJSON(providers.WebhookResponse{Events: []providers.WebhookEvent{}}); err != nil {
			pdk.SetError(err)
			return ExitTransient
		}
		return ExitSuccess
	}

	data := map[string]any{
		"to":     payload.To,
		"from":   payload.From,
		"status": payload.MessageStatus,
	}
	if payload.ErrorCode != "" {
		data["error_code"] = payload.ErrorCode
	}
	if payload.ErrorMessage != "" {
		data["error_message"] = payload.ErrorMessage
	}

	if err := pdk.OutputJSON(providers.WebhookResponse{
		Events: []providers.WebhookEvent{
			{
				EventName: eventName,
				MessageID: payload.MessageSid,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Data:      data,
			},
		},
	}); err != nil {
		pdk.SetError(err)
		return ExitTransient
	}
	return ExitSuccess
}

//go:export validate
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

	errs := provider.ValidateConfig(config)

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

//go:export install
func Init() int32 {
	var req modules.InstallRequest
	if err := pdk.InputJSON(&req); err != nil {
		pdk.SetError(err)
		return ExitPermanent
	}

	var config Config
	if err := json.Unmarshal(req.Config, &config); err != nil {
		pdk.SetError(fmt.Errorf("failed to parse config: %w", err))
		return ExitPermanent
	}

	if config.AccountSID == "" || config.AuthToken == "" {
		pdk.SetError(fmt.Errorf("missing required Account SID or Auth Token"))
		return ExitPermanent
	}

	state, err := json.Marshal(map[string]string{
		"webhookUrl": req.WebhookURL,
	})
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to marshal state: %w", err))
		return ExitTransient
	}

	pdk.Log(pdk.LogInfo, fmt.Sprintf("twilio install: storing webhook URL %s", req.WebhookURL))

	err = pdk.OutputJSON(modules.InstallResponse{
		State: state,
	})
	if err != nil {
		pdk.SetError(err)
		return ExitTransient
	}
	return ExitSuccess
}

//go:export uninstall
func Destroy() int32 {
	var req modules.UninstallRequest
	if err := pdk.InputJSON(&req); err != nil {
		pdk.SetError(err)
		return ExitPermanent
	}

	if err := pdk.OutputJSON(modules.UninstallResponse{}); err != nil {
		pdk.SetError(err)
		return ExitTransient
	}
	return ExitSuccess
}

func main() {}
