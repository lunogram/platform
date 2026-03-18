// Package providers implements a WASM-based provider system for messaging channels.
//
// It builds on the generic wasm package to provide provider-specific functionality
// including channel support validation and a typed Send interface for email, SMS,
// and push notifications.
//
// # WASM exit code convention
//
// Provider WASM modules return an int32 exit code from their send() function:
//
//   - 0: success
//   - -1: transient/retryable error (e.g., rate limit, network timeout)
//   - -2: permanent/non-retryable error (e.g., invalid recipient, validation failure)
//
// Any other non-zero exit code is treated as a transient error for backward
// compatibility.
package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"

	"go.uber.org/zap"

	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/wasm"
	"github.com/lunogram/platform/pkg/modules/providers"
)

// Provider wraps a WASM module with provider-specific functionality.
type Provider struct {
	*wasm.Module[providers.ProviderManifest]
}

// exitCodePermanent is the WASM exit code for permanent/non-retryable errors.
// WASM int32(-2) maps to uint32(0xFFFFFFFE).
const exitCodePermanent uint32 = 0xFFFFFFFE

// ProviderError is returned when a WASM provider's send() function fails.
// It carries the exit code so callers can distinguish permanent from transient failures.
type ProviderError struct {
	Code    uint32
	Message string
}

func (e *ProviderError) Error() string {
	return e.Message
}

// IsPermanent reports whether this provider error represents a permanent failure
// that should not be retried.
func (e *ProviderError) IsPermanent() bool {
	return e.Code == exitCodePermanent
}

// Send invokes the provider's send function.
func (p *Provider) Send(ctx context.Context, req *providers.SendRequest[map[string]any]) (*providers.SendResponse, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal send request: %w", err)
	}

	code, res, err := p.Call(ctx, "send", payload)
	if err != nil {
		return nil, fmt.Errorf("failed to call provider send: %w", err)
	}

	if code != 0 {
		return nil, &ProviderError{
			Code:    code,
			Message: fmt.Sprintf("provider send failed: %s", string(res)),
		}
	}

	var response providers.SendResponse
	if err := json.Unmarshal(res, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal send response: %w", err)
	}

	return &response, nil
}

// SupportsChannel checks if this provider supports a specific channel.
func (p *Provider) SupportsChannel(ch providers.Channel) bool {
	return slices.Contains(p.Manifest().Spec.Channels, ch)
}

// Registry is a provider-specific registry that wraps the generic WASM module registry.
type Registry struct {
	*wasm.Registry[providers.ProviderManifest]
}

// NewRegistry creates a new provider registry with the given configuration.
func NewRegistry(config config.WASM, logger *zap.Logger) *Registry {
	return &Registry{
		Registry: wasm.NewRegistry[providers.ProviderManifest](config, logger),
	}
}

// LoadFromFS loads all provider modules from an embedded filesystem.
// Validates that each loaded module supports at least one channel.
func (r *Registry) LoadFromFS(ctx context.Context, fsys fs.FS, dir string) error {
	if err := r.Registry.LoadFromFS(ctx, fsys, dir); err != nil {
		return err
	}

	for _, module := range r.Registry.All() {
		if len(module.Manifest().Spec.Channels) == 0 {
			return fmt.Errorf("provider %s must support at least one channel", module.Manifest().Metadata.ID)
		}
	}

	return nil
}

// Get retrieves a provider by ID.
func (r *Registry) Get(id string) (*Provider, bool) {
	module, exists := r.Registry.Get(id)
	if !exists {
		return nil, false
	}
	return &Provider{Module: module}, true
}

// All returns all registered providers.
func (r *Registry) All() []*Provider {
	modules := r.Registry.All()
	result := make([]*Provider, len(modules))
	for i, module := range modules {
		result[i] = &Provider{Module: module}
	}
	return result
}

// SupportsChannel checks if a module supports a specific channel.
func (r *Registry) SupportsChannel(moduleID string, channel providers.Channel) bool {
	provider, exists := r.Get(moduleID)
	if !exists {
		return false
	}
	return provider.SupportsChannel(channel)
}
