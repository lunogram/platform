package main

import (
	"github.com/extism/go-pdk"
	"github.com/lunogram/platform/pkg/modules"
	"github.com/lunogram/platform/pkg/modules/actions"
)

// Config is the action-specific configuration.
type Config struct {
	APIKey string `json:"api_key"`
}

//go:export manifest
func Manifest() int32 {
	manifest := actions.ActionManifest{
		Metadata: modules.Metadata{
			ID:          "test",
			Title:       "Test Action",
			Description: "Test action for WASM module testing",
			Tags:        []string{"test", "mock"},
		},
		Version: "1.0.0",
		License: "MIT",
		Author: modules.Author{
			Name:  "Lunogram",
			Email: "dev@lunogram.io",
			URL:   "https://lunogram.com",
		},
		Spec: actions.ActionSpec{
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
			Payload: &modules.JSONSchema{
				Type: "object",
				Properties: []modules.JSONSchemaProperty{
					{
						Name: "message",
						Schema: &modules.JSONSchema{
							Type:        "string",
							Description: "Test message",
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

//go:export execute
func Execute() int32 {
	var req actions.ExecuteRequest[Config]
	err := pdk.InputJSON(&req)
	if err != nil {
		pdk.SetError(err)
		return -1
	}

	response := actions.ExecuteResponse{
		Status: "completed",
		Metadata: map[string]any{
			"action":    "test",
			"api_key":   req.Config.APIKey,
			"payload":   string(req.Payload),
			"variables": req.Variables,
		},
	}

	err = pdk.OutputJSON(response)
	if err != nil {
		pdk.SetError(err)
		return -1
	}

	return 0
}

//go:export preview
func Preview() int32 {
	pdk.OutputString("<p>test</p>")
	return 0
}

func main() {}
