package main

import (
	"fmt"

	"github.com/extism/go-pdk"
	"github.com/lunogram/platform/pkg/modules"
	"github.com/lunogram/platform/pkg/modules/providers"
)

//go:export manifest
func Manifest() int32 {
	manifest := providers.ProviderManifest{
		Metadata: modules.Metadata{
			ID:          "twilio",
			Title:       "Twilio",
			Description: "Send emails and SMS via Twilio",
			Icon:        "https://static.cdnlogo.com/logos/t/14/twilio.svg",
			Color:       "#c7252b",
			Tags:        []string{"email", "sms"},
		},
		Website: "https://twilio.com",
		Version: "1.0.0",
		License: "MIT",
		Author: modules.Author{
			Name:  "Lunogram",
			Email: "dev@lunogram.io",
			URL:   "https://lunogram.com",
		},
		Spec: providers.ProviderSpec{
			Channels: []providers.Channel{
				providers.ChannelEmail,
				providers.ChannelSMS,
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
							},
							Required: []string{"accountSid", "authToken"},
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
	AccountSID string `json:"accountSid"`
	AuthToken  string `json:"authToken"`
}

//go:export send
func Send() int32 {
	var req providers.SendRequest[Config]
	err := pdk.InputJSON(&req)
	if err != nil {
		pdk.SetError(err)
		return -1
	}

	switch req.Channel {
	case providers.ChannelEmail:
		return sendEmail(&req)

	case providers.ChannelSMS:
		return sendSMS(&req)

	default:
		pdk.SetError(fmt.Errorf("unsupported channel: %s", req.Channel))
		return -1
	}
}

func sendEmail(req *providers.SendRequest[Config]) int32 {
	email, err := req.GetEmailPayload()
	if err != nil {
		pdk.SetError(err)
		return -1
	}

	pdk.Log(pdk.LogInfo, "sending email via Twilio SendGrid")
	pdk.Log(pdk.LogInfo, fmt.Sprintf("to: %s", email.To))
	pdk.Log(pdk.LogInfo, fmt.Sprintf("subject: %s", email.Subject))

	// TODO: Implement Twilio SendGrid API call
	// See: https://docs.sendgrid.com/api-reference/mail-send/mail-send

	response := providers.SendResponse{
		ID:     "twilio-email-123",
		Status: "sent",
		Metadata: map[string]any{
			"channel": req.Channel,
		},
	}

	err = pdk.OutputJSON(response)
	if err != nil {
		pdk.SetError(err)
		return -1
	}

	return 0
}

func sendSMS(req *providers.SendRequest[Config]) int32 {
	sms, err := req.GetSMSPayload()
	if err != nil {
		pdk.SetError(err)
		return -1
	}

	pdk.Log(pdk.LogInfo, "sending SMS via Twilio")
	pdk.Log(pdk.LogInfo, fmt.Sprintf("to: %s", sms.To))
	pdk.Log(pdk.LogInfo, fmt.Sprintf("from: %s", sms.From))
	pdk.Log(pdk.LogInfo, fmt.Sprintf("body: %s", sms.Body))

	// TODO: Implement Twilio SMS API call
	// See: https://www.twilio.com/docs/sms/api/message-resource

	response := providers.SendResponse{
		ID:     "twilio-sms-456",
		Status: "sent",
		Metadata: map[string]any{
			"channel": req.Channel,
		},
	}

	err = pdk.OutputJSON(response)
	if err != nil {
		pdk.SetError(err)
		return -1
	}

	return 0
}

func main() {}
