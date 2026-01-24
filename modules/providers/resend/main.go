package main

import (
	"fmt"
	"net/http"

	"github.com/extism/go-pdk"
	pdkhttp "github.com/extism/go-pdk/http"
	"github.com/lunogram/platform/pkg/modules"
	"github.com/lunogram/platform/pkg/modules/providers"
	"github.com/resend/resend-go/v3"
)

//go:export manifest
func Manifest() int32 {
	manifest := providers.ProviderManifest{
		Metadata: modules.Metadata{
			ID:          "resend",
			Title:       "Resend Email",
			Description: "Resend email service integration",
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
				Properties: map[string]*modules.JSONSchema{
					"data": {
						Type: "object",
						Properties: map[string]*modules.JSONSchema{
							"apiKey": {Type: "string", Title: "Resend API Key", Format: "password"},
							"from":   {Type: "string", Title: "Default From Address", Description: "Default 'from' email address"},
						},
						Required: []string{"apiKey"},
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
		Transport: &pdkhttp.HTTPTransport{},
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
