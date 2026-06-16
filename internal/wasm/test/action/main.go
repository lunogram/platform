package main

import (
	"encoding/json"

	"github.com/extism/go-pdk"
	"github.com/lunogram/platform/pkg/modules"
	actiontypes "github.com/lunogram/platform/pkg/modules/actions"
)

// Config is the action-specific configuration.
type Config struct {
	APIKey string `json:"api_key"`
}

//go:export manifest
func Manifest() int32 {
	manifest := modules.IntegrationManifest{
		APIVersion: "v1",
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
		Capabilities: []modules.Capability{
			{
				Type:    "actions",
				Version: "v1",
				Spec: mustMarshalJSON(modules.ActionsSpec{
					Functions: []modules.ActionFunction{
						{
							ID:          "run",
							Title:       "Run Test",
							Description: "Execute test action",
							Input: &modules.JSONSchema{
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
					},
				}),
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

//go:export validate
func Validate() int32 {
	var req modules.ValidateRequest
	err := pdk.InputJSON(&req)
	if err != nil {
		pdk.SetError(err)
		return -1
	}

	var config Config
	if err := json.Unmarshal(req.Config, &config); err != nil {
		pdk.SetError(err)
		return -1
	}

	statusCode := 200
	message := "Configuration is valid"

	if config.APIKey == "" {
		statusCode = 400
		message = "API key is required"
	}

	response := modules.ValidateResponse{
		Valid:   statusCode < 400,
		Message: message,
	}

	err = pdk.OutputJSON(response)
	if err != nil {
		pdk.SetError(err)
		return -1
	}

	return 0
}

//go:export action_run
func Run() int32 {
	var req actiontypes.ExecuteRequest[Config]
	err := pdk.InputJSON(&req)
	if err != nil {
		pdk.SetError(err)
		return -1
	}

	response := actiontypes.ExecuteResponse{
		StatusCode: 200,
		Metadata: map[string]any{
			"action":  "test",
			"api_key": req.Config.APIKey,
			"input":   string(req.Input),
		},
	}

	err = pdk.OutputJSON(response)
	if err != nil {
		pdk.SetError(err)
		return -1
	}

	return 0
}

//go:export action_preview_run
func Preview() int32 {
	pdk.OutputString("<p>test</p>")
	return 0
}

func main() {}

func mustMarshalJSON(v any) json.RawMessage {
	spec, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	return spec
}
