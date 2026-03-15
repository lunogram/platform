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
			ID:          "logger",
			Title:       "Logger",
			Description: "Logger channel integration for debugging",
			Icon:        "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCIgd2lkdGg9IjI0IiBoZWlnaHQ9IjI0IiBmaWxsPSJub25lIiBzdHJva2U9IiMwMDAwMDAiIHN0cm9rZS13aWR0aD0iMiIgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIiBzdHJva2UtbGluZWpvaW49InJvdW5kIiBzdHlsZT0ib3BhY2l0eToxOyI+PHBhdGggZmlsbD0ibm9uZSIgICAgIGQ9Ik00IDEyaC4wMU00IDZoLjAxTTQgMThoLjAxTTggMThoMm0tMi02aDJNOCA2aDJtNCAwaDZtLTYgNmg2bS02IDZoNiIvPjwvc3ZnPg==",
			Color:       "#000000",
			Tags:        []string{"logging", "debug"},
		},
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
				providers.ChannelPush,
			},
			Config: &modules.JSONSchema{
				Type: "object",
				Properties: []modules.JSONSchemaProperty{
					{
						Name: "data",
						Schema: &modules.JSONSchema{
							Type:       "object",
							Properties: []modules.JSONSchemaProperty{},
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

type Config struct{}

//go:export send
func Send() int32 {
	var req providers.SendRequest[Config]
	err := pdk.InputJSON(&req)
	if err != nil {
		pdk.SetError(err)
		return -1
	}

	pdk.Log(pdk.LogInfo, fmt.Sprintf("channel: %s", req.Channel))

	switch req.Channel {
	case providers.ChannelEmail:
		email, err := req.GetEmailPayload()
		if err != nil {
			pdk.SetError(err)
			return -1
		}
		pdk.Log(pdk.LogInfo, fmt.Sprintf("to: %s", email.To))
		pdk.Log(pdk.LogInfo, fmt.Sprintf("from: %s <%s>", email.From.Name, email.From.Address))
		pdk.Log(pdk.LogInfo, fmt.Sprintf("subject: %s", email.Subject))
		pdk.Log(pdk.LogInfo, fmt.Sprintf("html: %s", email.HTML))

	case providers.ChannelSMS:
		sms, err := req.GetSMSPayload()
		if err != nil {
			pdk.SetError(err)
			return -1
		}
		pdk.Log(pdk.LogInfo, fmt.Sprintf("to: %s", sms.To))
		pdk.Log(pdk.LogInfo, fmt.Sprintf("from: %s", sms.From))
		pdk.Log(pdk.LogInfo, fmt.Sprintf("body: %s", sms.Body))

	case providers.ChannelPush:
		push, err := req.GetPushPayload()
		if err != nil {
			pdk.SetError(err)
			return -1
		}
		pdk.Log(pdk.LogInfo, fmt.Sprintf("tokens: %v", push.Tokens))
		pdk.Log(pdk.LogInfo, fmt.Sprintf("title: %s", push.Title))
		pdk.Log(pdk.LogInfo, fmt.Sprintf("body: %s", push.Body))

	default:
		err := fmt.Errorf("unsupported channel: %s", req.Channel)
		pdk.SetError(err)
		return -1
	}

	response := providers.SendResponse{
		Status: "logged",
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
