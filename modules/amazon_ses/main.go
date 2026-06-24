package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/extism/go-pdk"
	pdkhttp "github.com/extism/go-pdk/http"
	"github.com/lunogram/platform/pkg/modules"
	"github.com/lunogram/platform/pkg/modules/providers"
)

type SESv2Client struct {
	Config Config
	client *http.Client
}

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

func NewSESv2Client(cfg Config) *SESv2Client {
	httpClient := &http.Client{
		Transport: &safeTransport{inner: &pdkhttp.HTTPTransport{}},
	}
	return &SESv2Client{
		Config: cfg,
		client: httpClient,
	}
}

// providerCapabilitySpec describes the provider capability advertised by this
// integration. SES delivers via the SESv2 API; delivery/bounce notifications
// arrive out-of-band via SNS, which the platform does not yet ingest, so
// Webhook is left false.
func providerCapabilitySpec() json.RawMessage {
	spec, err := json.Marshal(modules.ProviderSpec{
		Channels: []modules.Channel{modules.ChannelEmail},
		Webhook:  false,
		RateLimit: &modules.RateLimit{
			Limit:    14,
			Interval: "1s",
			Override: true,
		},
	})
	if err != nil {
		panic(err)
	}

	return spec
}

func (c *SESv2Client) SendEmail(req *SESv2SendEmailRequest) (string, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	endpoint := "https://email." + c.Config.Region + ".amazonaws.com/v2/email/outbound-emails"
	httpReq, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Add V4 Signature
	SignV4(httpReq, body, c.Config.AccessKeyID, c.Config.SecretAccessKey, c.Config.SessionToken, c.Config.Region, "ses", time.Now().UTC())

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("ses error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var res struct {
		MessageId string `json:"MessageId"`
	}
	_ = json.Unmarshal(respBody, &res)
	return res.MessageId, nil
}

//go:export manifest
func Manifest() int32 {
	manifest := modules.IntegrationManifest{
		APIVersion: "v1",
		Metadata: modules.Metadata{
			ID:          "amazon_ses",
			Title:       "Amazon SES",
			Description: "Amazon Simple Email Service provider",
			Icon:        "https://cdn.lunogram.com/icons/amazon-ses.svg",
			Color:       "#FF9900",
			Tags:        []string{"email"},
		},
		Website: "https://aws.amazon.com/ses/",
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
								Name:   "accessKeyId",
								Schema: &modules.JSONSchema{Type: "string", Title: "Access Key ID"},
							},
							{
								Name:   "secretAccessKey",
								Schema: &modules.JSONSchema{Type: "string", Title: "Secret Access Key", Format: "password"},
							},
							{
								Name:   "region",
								Schema: &modules.JSONSchema{Type: "string", Title: "AWS Region", Description: "For example us-east-1"},
							},
							{
								Name:   "sessionToken",
								Schema: &modules.JSONSchema{Type: "string", Title: "Session Token", Description: "Optional STS Session Token", Format: "password"},
							},
							{
								Name:   "configurationSet",
								Schema: &modules.JSONSchema{Type: "string", Title: "Configuration Set", Description: "Optional SES Configuration Set name"},
							},
						},
						Required: []string{"accessKeyId", "secretAccessKey", "region"},
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

	if req.Config.AccessKeyID == "" || req.Config.SecretAccessKey == "" || req.Config.Region == "" {
		pdk.SetError(fmt.Errorf("missing required config: accessKeyId, secretAccessKey, and region are required"))
		return ExitPermanent
	}

	client := NewSESv2Client(req.Config)
	sendReq := ComposeSendEmailRequest(email, req.Config)

	// Log the outgoing request payload for debugging
	debugPayload, _ := json.Marshal(struct {
		From    string   `json:"from"`
		To      []string `json:"to"`
		Subject string   `json:"subject"`
		HasHTML bool     `json:"has_html"`
		HasText bool     `json:"has_text"`
	}{
		From:    sendReq.FromEmailAddress,
		To:      sendReq.Destination.ToAddresses,
		Subject: sendReq.Content.Simple.Subject.Data,
		HasHTML: sendReq.Content.Simple.Body.Html != nil,
		HasText: sendReq.Content.Simple.Body.Text != nil,
	})
	pdk.Log(pdk.LogDebug, fmt.Sprintf("amazon_ses send request: %s", string(debugPayload)))

	messageId, err := client.SendEmail(sendReq)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to send email via SES: %w", err))
		return classifyError(err)
	}

	err = pdk.OutputJSON(providers.SendResponse{
		ID:     messageId,
		Status: "sent",
	})
	if err != nil {
		pdk.SetError(err)
		return -1
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

	// Not implemented for SES yet: SNS integration requires parsing the SNS
	// notification that wraps SES events. The manifest advertises Webhook:
	// false, so this export should not normally be invoked.
	pdk.Log(pdk.LogDebug, "amazon_ses webhook handler called but not implemented")

	err := pdk.OutputJSON(providers.WebhookResponse{Events: []providers.WebhookEvent{}})
	if err != nil {
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

func main() {}
