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
			ID:          "testprovider",
			Title:       "Test Provider",
			Description: "Test provider for WASM module testing",
			Tags:        []string{"test", "mock"},
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
						Name: "api_key",
						Schema: &modules.JSONSchema{
							Type:        "string",
							Description: "Test API key",
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
	APIKey string `json:"api_key"`
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
		_, err := req.GetEmailPayload()
		if err != nil {
			pdk.SetError(err)
			return -1
		}

	case providers.ChannelSMS:
		_, err := req.GetSMSPayload()
		if err != nil {
			pdk.SetError(err)
			return -1
		}

	case providers.ChannelPush:
		_, err := req.GetPushPayload()
		if err != nil {
			pdk.SetError(err)
			return -1
		}

	default:
		err := fmt.Errorf("unsupported channel: %s", req.Channel)
		pdk.SetError(err)
		return -1
	}

	response := providers.SendResponse{
		Status: "sent",
		Metadata: map[string]any{
			"channel":  req.Channel,
			"provider": "testprovider",
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
