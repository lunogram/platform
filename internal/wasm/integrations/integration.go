// Package integrations implements a WASM-based unified integration system.
//
// It builds on the generic wasm package to provide integration-specific
// functionality including provider messaging, action execution, and lifecycle
// management within a single module.
package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lunogram/platform/internal/wasm"
	"github.com/lunogram/platform/pkg/modules"
	actiontypes "github.com/lunogram/platform/pkg/modules/actions"
	providertypes "github.com/lunogram/platform/pkg/modules/providers"
)

// Integration wraps a WASM module with unified integration functionality.
type Integration struct {
	*wasm.Module[modules.IntegrationManifest]
}

// exitCodePermanent is the WASM exit code for permanent/non-retryable errors.
// WASM int32(-2) maps to uint32(0xFFFFFFFE).
const exitCodePermanent uint32 = 0xFFFFFFFE

// ProviderError is returned when a provider send or webhook call fails.
// It carries the exit code so callers can distinguish permanent from transient failures.
type ProviderError struct {
	Code    uint32
	Message string
}

func (e *ProviderError) Error() string {
	return e.Message
}

// IsPermanent reports whether this provider error represents a permanent failure.
func (e *ProviderError) IsPermanent() bool {
	return e.Code == exitCodePermanent
}

func (i *Integration) callAny(ctx context.Context, exports []string, input []byte) (string, uint32, []byte, error) {
	for _, name := range exports {
		if !i.FunctionExists(name) {
			continue
		}

		code, res, err := i.Call(ctx, name, input)
		return name, code, res, err
	}

	return "", 0, nil, fmt.Errorf("no compatible export found (tried: %s)", strings.Join(exports, ", "))
}

// HasCapability checks if any capability in the manifest has the given type.
func (i *Integration) HasCapability(capType string) bool {
	for _, cap := range i.Manifest().Capabilities {
		if cap.Type == capType {
			return true
		}
	}
	return false
}

// HasProvider reports whether this integration declares a provider capability.
func (i *Integration) HasProvider() bool {
	return i.HasCapability("provider")
}

// HasActions reports whether this integration declares an actions capability.
func (i *Integration) HasActions() bool {
	return i.HasCapability("actions")
}

// ProviderSpec finds and decodes the provider capability spec.
func (i *Integration) ProviderSpec() (*modules.ProviderSpec, bool) {
	for _, cap := range i.Manifest().Capabilities {
		if cap.Type != "provider" {
			continue
		}

		var spec modules.ProviderSpec
		if err := json.Unmarshal(cap.Spec, &spec); err != nil {
			return nil, false
		}

		return &spec, true
	}

	return nil, false
}

// ActionsSpec finds and decodes the actions capability spec.
func (i *Integration) ActionsSpec() (*modules.ActionsSpec, bool) {
	for _, cap := range i.Manifest().Capabilities {
		if cap.Type != "actions" {
			continue
		}

		var spec modules.ActionsSpec
		if err := json.Unmarshal(cap.Spec, &spec); err != nil {
			return nil, false
		}

		return &spec, true
	}

	return nil, false
}

// Validate calls the module's validate export to check configuration.
// Supports both integration and compatibility response formats.
func (i *Integration) Validate(ctx context.Context, req modules.ValidateRequest) (*modules.ValidateResponse, error) {
	if !i.FunctionExists("validate") {
		return &modules.ValidateResponse{Valid: true}, nil
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal validate request: %w", err)
	}

	code, res, err := i.Call(ctx, "validate", payload)
	if err != nil {
		return nil, fmt.Errorf("failed to call integration validate: %w", err)
	}

	if code != 0 {
		return nil, fmt.Errorf("integration validate failed (code %d): %s", code, string(res))
	}

	var response modules.ValidateResponse
	if err := json.Unmarshal(res, &response); err == nil {
		if response.Valid || len(response.Errors) > 0 || response.Message != "" {
			return &response, nil
		}
	}

	var actionResponse actiontypes.ValidateResponse
	if err := json.Unmarshal(res, &actionResponse); err == nil {
		if actionResponse.StatusCode != 0 || actionResponse.Message != "" {
			return &modules.ValidateResponse{
				Valid:   actionResponse.StatusCode < 400,
				Message: actionResponse.Message,
			}, nil
		}
	}

	return nil, fmt.Errorf("failed to decode validate response")
}

// Install calls the module's install export for first-time setup.
// Falls back to provider init() when install() is unavailable.
func (i *Integration) Install(ctx context.Context, req modules.InstallRequest) (*modules.InstallResponse, error) {
	if i.FunctionExists("install") {
		payload, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal install request: %w", err)
		}

		code, res, err := i.Call(ctx, "install", payload)
		if err != nil {
			return nil, fmt.Errorf("failed to call integration install: %w", err)
		}

		if code != 0 {
			return nil, fmt.Errorf("integration install failed (code %d): %s", code, string(res))
		}

		var response modules.InstallResponse
		if err := json.Unmarshal(res, &response); err != nil {
			return nil, fmt.Errorf("failed to unmarshal install response: %w", err)
		}

		return &response, nil
	}

	if !i.FunctionExists("init") {
		return &modules.InstallResponse{}, nil
	}

	compatReq := providertypes.InitRequest{
		Config:     req.Config,
		WebhookURL: req.WebhookURL,
		ProviderID: req.IntegrationID,
		ProjectID:  req.ProjectID,
	}
	payload, err := json.Marshal(compatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal init request: %w", err)
	}

	code, res, err := i.Call(ctx, "init", payload)
	if err != nil {
		return nil, fmt.Errorf("failed to call init: %w", err)
	}

	if code != 0 {
		return nil, fmt.Errorf("init failed (code %d): %s", code, string(res))
	}

	var response providertypes.InitResponse
	if err := json.Unmarshal(res, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal init response: %w", err)
	}

	return &modules.InstallResponse{State: response.ConfigPatch}, nil
}

// Upgrade calls the module's upgrade export when configuration changes.
// Falls back to install behavior when upgrade() is unavailable.
func (i *Integration) Upgrade(ctx context.Context, req modules.UpgradeRequest) (*modules.UpgradeResponse, error) {
	if i.FunctionExists("upgrade") {
		payload, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal upgrade request: %w", err)
		}

		code, res, err := i.Call(ctx, "upgrade", payload)
		if err != nil {
			return nil, fmt.Errorf("failed to call integration upgrade: %w", err)
		}

		if code != 0 {
			return nil, fmt.Errorf("integration upgrade failed (code %d): %s", code, string(res))
		}

		var response modules.UpgradeResponse
		if err := json.Unmarshal(res, &response); err != nil {
			return nil, fmt.Errorf("failed to unmarshal upgrade response: %w", err)
		}

		return &response, nil
	}

	installRes, err := i.Install(ctx, modules.InstallRequest{
		Config:        req.Config,
		WebhookURL:    req.WebhookURL,
		IntegrationID: req.IntegrationID,
		ProjectID:     req.ProjectID,
	})
	if err != nil {
		return nil, err
	}

	return &modules.UpgradeResponse{State: installRes.State}, nil
}

// Uninstall calls the module's uninstall export for cleanup.
// Falls back to provider destroy() when uninstall() is unavailable.
func (i *Integration) Uninstall(ctx context.Context, req modules.UninstallRequest) (*modules.UninstallResponse, error) {
	if i.FunctionExists("uninstall") {
		payload, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal uninstall request: %w", err)
		}

		code, res, err := i.Call(ctx, "uninstall", payload)
		if err != nil {
			return nil, fmt.Errorf("failed to call integration uninstall: %w", err)
		}

		if code != 0 {
			return nil, fmt.Errorf("integration uninstall failed (code %d): %s", code, string(res))
		}

		var response modules.UninstallResponse
		if err := json.Unmarshal(res, &response); err != nil {
			return nil, fmt.Errorf("failed to unmarshal uninstall response: %w", err)
		}

		return &response, nil
	}

	if !i.FunctionExists("destroy") {
		return &modules.UninstallResponse{}, nil
	}

	compatReq := providertypes.DestroyRequest{
		Config:     req.Config,
		ProviderID: req.IntegrationID,
		ProjectID:  req.ProjectID,
	}
	payload, err := json.Marshal(compatReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal destroy request: %w", err)
	}

	code, res, err := i.Call(ctx, "destroy", payload)
	if err != nil {
		return nil, fmt.Errorf("failed to call destroy: %w", err)
	}

	if code != 0 {
		return nil, fmt.Errorf("destroy failed (code %d): %s", code, string(res))
	}

	return &modules.UninstallResponse{}, nil
}

// Send invokes provider send using canonical or compatibility export names.
func (i *Integration) Send(ctx context.Context, req providertypes.SendRequest[map[string]any]) (*providertypes.SendResponse, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal send request: %w", err)
	}

	_, code, res, err := i.callAny(ctx, []string{"provider_send", "send"}, payload)
	if err != nil {
		return nil, err
	}

	if code != 0 {
		return nil, &ProviderError{Code: code, Message: string(res)}
	}

	var response providertypes.SendResponse
	if err := json.Unmarshal(res, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal send response: %w", err)
	}

	return &response, nil
}

// Webhook invokes provider webhook using canonical or compatibility export names.
func (i *Integration) Webhook(ctx context.Context, req providertypes.WebhookRequest) (*providertypes.WebhookResponse, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal webhook request: %w", err)
	}

	_, code, res, err := i.callAny(ctx, []string{"provider_webhook", "webhook"}, payload)
	if err != nil {
		return nil, err
	}

	if code != 0 {
		return nil, &ProviderError{Code: code, Message: string(res)}
	}

	var response providertypes.WebhookResponse
	if err := json.Unmarshal(res, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal webhook response: %w", err)
	}

	return &response, nil
}

// HasWebhook reports whether the integration exports a webhook handler.
func (i *Integration) HasWebhook() bool {
	return i.FunctionExists("provider_webhook") || i.FunctionExists("webhook")
}

// Execute invokes an action function export.
func (i *Integration) Execute(ctx context.Context, functionID string, req *actiontypes.ExecuteRequest[json.RawMessage]) (*actiontypes.ExecuteResponse, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal execute request: %w", err)
	}

	if functionID == "" {
		return nil, fmt.Errorf("action function id is required")
	}

	name, code, res, err := i.callAny(ctx, []string{"action_" + functionID, functionID}, payload)
	if err != nil {
		return nil, err
	}

	if code != 0 {
		return nil, fmt.Errorf("action function %q returned code %d: %s", name, code, string(res))
	}

	var response actiontypes.ExecuteResponse
	if err := json.Unmarshal(res, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal execute response for %q: %w", name, err)
	}

	return &response, nil
}

// Preview invokes an action preview export and returns raw HTML bytes.
func (i *Integration) Preview(ctx context.Context, functionID string) ([]byte, error) {
	var exports []string
	if functionID != "" {
		exports = append(exports, "action_preview_"+functionID)
	}
	exports = append(exports, "preview")
	if functionID != "" {
		exports = append(exports, functionID+"_preview")
	}

	name, code, res, err := i.callAny(ctx, exports, nil)
	if err != nil {
		return nil, err
	}

	if code != 0 {
		return nil, fmt.Errorf("%q returned code %d: %s", name, code, string(res))
	}

	return res, nil
}

// HasPreview reports whether this module exports an action preview function.
func (i *Integration) HasPreview(functionID string) bool {
	if functionID != "" && i.FunctionExists("action_preview_"+functionID) {
		return true
	}

	return i.FunctionExists("preview")
}
