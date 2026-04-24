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
	if err := json.Unmarshal(res, &response); err != nil {
		return nil, fmt.Errorf("failed to decode validate response: %w", err)
	}

	return &response, nil
}

// Install calls the module's install export for first-time setup.
func (i *Integration) Install(ctx context.Context, req modules.InstallRequest) (*modules.InstallResponse, error) {
	if !i.FunctionExists("install") {
		return &modules.InstallResponse{}, nil
	}

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
func (i *Integration) Uninstall(ctx context.Context, req modules.UninstallRequest) (*modules.UninstallResponse, error) {
	if !i.FunctionExists("uninstall") {
		return &modules.UninstallResponse{}, nil
	}

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

// Send invokes provider send.
func (i *Integration) Send(ctx context.Context, req providertypes.SendRequest[map[string]any]) (*providertypes.SendResponse, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal send request: %w", err)
	}

	if !i.FunctionExists("provider_send") {
		return nil, fmt.Errorf("module does not export provider_send")
	}

	code, res, err := i.Call(ctx, "provider_send", payload)
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

// Webhook invokes provider webhook.
func (i *Integration) Webhook(ctx context.Context, req providertypes.WebhookRequest) (*providertypes.WebhookResponse, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal webhook request: %w", err)
	}

	if !i.FunctionExists("provider_webhook") {
		return nil, fmt.Errorf("module does not export provider_webhook")
	}

	code, res, err := i.Call(ctx, "provider_webhook", payload)
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
	return i.FunctionExists("provider_webhook")
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

	exportName := "action_" + functionID
	if !i.FunctionExists(exportName) {
		return nil, fmt.Errorf("module does not export %q", exportName)
	}

	code, res, err := i.Call(ctx, exportName, payload)
	if err != nil {
		return nil, err
	}

	if code != 0 {
		return nil, fmt.Errorf("action function %q returned code %d: %s", exportName, code, string(res))
	}

	var response actiontypes.ExecuteResponse
	if err := json.Unmarshal(res, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal execute response for %q: %w", exportName, err)
	}

	return &response, nil
}

// Preview invokes an action preview export and returns raw HTML bytes.
func (i *Integration) Preview(ctx context.Context, functionID string) ([]byte, error) {
	exportName := "preview"
	if functionID != "" {
		exportName = "action_preview_" + functionID
	}

	if !i.FunctionExists(exportName) {
		return nil, fmt.Errorf("module does not export %q", exportName)
	}

	code, res, err := i.Call(ctx, exportName, nil)
	if err != nil {
		return nil, err
	}

	if code != 0 {
		return nil, fmt.Errorf("%q returned code %d: %s", exportName, code, string(res))
	}

	return res, nil
}

// HasPreview reports whether this module exports an action preview function.
func (i *Integration) HasPreview(functionID string) bool {
	if functionID != "" && i.FunctionExists("action_preview_"+functionID) {
		return true
	}
	if functionID != "" {
		return false
	}

	return i.FunctionExists("preview")
}
