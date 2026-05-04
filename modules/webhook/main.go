package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/extism/go-pdk"
	pdkhttp "github.com/extism/go-pdk/http"
	"github.com/lunogram/platform/pkg/modules"
	actiontypes "github.com/lunogram/platform/pkg/modules/actions"
)

//go:embed preview/index.html
var previewHTML []byte

// WebhookConfig is currently empty — authentication is handled via headers in the payload.
type WebhookConfig struct{}

// WebhookInput defines the HTTP request to make.
type WebhookInput struct {
	Method   string            `json:"method"`
	Endpoint string            `json:"endpoint"`
	Headers  map[string]string `json:"headers,omitempty"`
	Body     string            `json:"body,omitempty"`
}

//go:export manifest
func Manifest() int32 {
	manifest := modules.IntegrationManifest{
		APIVersion: "v1",
		Metadata: modules.Metadata{
			ID:          "webhook",
			Title:       "Webhook",
			Description: "Send an HTTP request to an external endpoint",
			Icon:        "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCIgd2lkdGg9IjI0IiBoZWlnaHQ9IjI0IiBmaWxsPSJub25lIiBzdHJva2U9IiMwMDAwMDAiIHN0cm9rZS13aWR0aD0iMiIgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIiBzdHJva2UtbGluZWpvaW49InJvdW5kIiBzdHlsZT0ib3BhY2l0eToxOyI+PHBhdGggZD0iTTE4IDE2Ljk4aC01Ljk5Yy0xLjEgMC0xLjk1Ljk0LTIuNDggMS45QTQgNCAwIDAgMSAyIDE3Yy4wMS0uNy4yLTEuNC41Ny0yIi8+PHBhdGggZD0ibTYgMTdsMy4xMy01Ljc4Yy41My0uOTcuMS0yLjE4LS41LTMuMWE0IDQgMCAxIDEgNi44OS00LjA2Ii8+PHBhdGggZD0ibTEyIDZsMy4xMyA1LjczQzE1LjY2IDEyLjcgMTYuOSAxMyAxOCAxM2E0IDQgMCAwIDEgMCA4Ii8+PC9zdmc+",
			Color:       "#fecc21",
			Tags:        []string{"http", "webhook", "integration"},
		},
		Version: "1.0.0",
		License: "MIT",
		Author: modules.Author{
			Name:  "Lunogram",
			Email: "dev@lunogram.io",
			URL:   "https://lunogram.com",
		},
		Capabilities: []modules.Capability{
			{
				Type:    "actions",
				Version: "v1",
				Spec: mustMarshalJSON(modules.ActionsSpec{
					Functions: []modules.ActionFunction{
						{
							ID:          "send_request",
							Title:       "Send Request",
							Description: "Send an HTTP request to an external endpoint",
							Input: &modules.JSONSchema{
								Type: "object",
								Properties: []modules.JSONSchemaProperty{
									{
										Name: "method",
										Schema: &modules.JSONSchema{
											Type:        "string",
											Title:       "HTTP Method",
											Description: "HTTP method for the request",
											Enum:        []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
										},
									},
									{
										Name: "endpoint",
										Schema: &modules.JSONSchema{
											Type:        "string",
											Title:       "Endpoint URL",
											Description: "URL to send the request to",
											Preview:     "https://api.example.com/webhook",
										},
									},
									{
										Name: "headers",
										Schema: &modules.JSONSchema{
											Type:        "object",
											Title:       "Headers",
											Description: "HTTP headers to include in the request",
											Format:      "key-value",
										},
									},
									{
										Name: "body",
										Schema: &modules.JSONSchema{
											Type:        "string",
											Title:       "Request Body",
											Description: "Body of the HTTP request (supports JSON, XML, or plain text)",
											Format:      "code",
										},
									},
								},
								Required: []string{"method", "endpoint"},
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
		pdk.SetError(fmt.Errorf("failed to parse validate request: %w", err))
		return -1
	}

	// The webhook module has no module-level config to validate (authentication
	// is handled via headers in the payload), so validation always succeeds.
	response := modules.ValidateResponse{
		Valid:   true,
		Message: "Webhook configuration is valid",
	}

	if err := pdk.OutputJSON(response); err != nil {
		pdk.SetError(err)
		return -1
	}

	return 0
}

//go:export action_send_request
func SendRequest() int32 {
	var req actiontypes.ExecuteRequest[WebhookConfig]
	err := pdk.InputJSON(&req)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to parse execute request: %w", err))
		return -1
	}

	// Unmarshal the input.
	var payload WebhookInput
	if err := json.Unmarshal(req.Input, &payload); err != nil {
		pdk.SetError(fmt.Errorf("failed to parse webhook input: %w", err))
		return -1
	}

	// Validate required fields.
	if payload.Method == "" {
		pdk.SetError(fmt.Errorf("missing required field: method"))
		return -1
	}
	if payload.Endpoint == "" {
		pdk.SetError(fmt.Errorf("missing required field: endpoint"))
		return -1
	}

	endpoint := payload.Endpoint
	body := payload.Body

	headers := make(map[string]string, len(payload.Headers))
	for k, v := range payload.Headers {
		headers[k] = v
	}

	// Build the HTTP request.
	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	httpClient := &http.Client{
		Transport: &pdkhttp.HTTPTransport{},
	}

	var httpReq *http.Request
	if bodyReader != nil {
		httpReq, err = http.NewRequest(payload.Method, endpoint, bodyReader)
	} else {
		httpReq, err = http.NewRequest(payload.Method, endpoint, nil)
	}
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to create HTTP request: %w", err))
		return -1
	}

	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}

	pdk.Log(pdk.LogInfo, fmt.Sprintf("executing webhook: %s %s", payload.Method, endpoint))

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		pdk.SetError(fmt.Errorf("webhook request failed: %w", err))
		return -1
	}
	if resp.Body != nil {
		defer resp.Body.Close()
	}

	// Read response body (cap at 64KB to avoid memory issues in WASM).
	var respBody []byte
	if resp.Body != nil {
		const maxBodySize = 64 * 1024
		limitedReader := io.LimitReader(resp.Body, maxBodySize)
		respBody, err = io.ReadAll(limitedReader)
		if err != nil {
			pdk.Log(pdk.LogWarn, fmt.Sprintf("failed to read response body: %v", err))
		}
	}

	// Collect response headers.
	respHeaders := make(map[string]string, len(resp.Header))
	for k, v := range resp.Header {
		respHeaders[k] = strings.Join(v, ", ")
	}

	statusCode := resp.StatusCode

	// Try to decode the response body as JSON so it's returned as a
	// structured object rather than an escaped string.
	var parsedBody any
	if err := json.Unmarshal(respBody, &parsedBody); err != nil {
		// Not valid JSON — fall back to the raw string.
		parsedBody = string(respBody)
	}

	response := actiontypes.ExecuteResponse{
		StatusCode: statusCode,
		Metadata: map[string]any{
			"method":   payload.Method,
			"endpoint": endpoint,
			"body":     parsedBody,
			"headers":  respHeaders,
		},
	}

	if err := pdk.OutputJSON(response); err != nil {
		pdk.SetError(err)
		return -1
	}

	return 0
}

//go:export action_preview_send_request
func Preview() int32 {
	pdk.Output(previewHTML)
	return 0
}

func mustMarshalJSON(v any) json.RawMessage {
	spec, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	return spec
}

func main() {}
