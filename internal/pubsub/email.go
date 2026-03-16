package pubsub

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/pubsub/schemas"
)

// EmailRenderer wraps a NATS caller to provide compile and render
// operations for React Email templates via the Deno renderer service.
type EmailRenderer struct {
	caller Caller
}

// NewEmailRenderer creates a new EmailRenderer backed by the given NATS caller.
func NewEmailRenderer(caller Caller) *EmailRenderer {
	return &EmailRenderer{caller: caller}
}

// Compile sends React Email JSX source to the Deno renderer service
// via NATS request/reply and returns the compiled JS bundle.
func (r *EmailRenderer) Compile(ctx context.Context, projectID uuid.UUID, source string) (string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	reply, err := r.caller.Call(ctx, schemas.EmailCompile(projectID), schemas.CompileEmail{
		Source: source,
	})
	if err != nil {
		return "", "", fmt.Errorf("compile email: %w", err)
	}

	var resp schemas.CompileEmailResponse
	if err := json.Unmarshal(reply, &resp); err != nil {
		return "", "", fmt.Errorf("unmarshal compile response: %w", err)
	}

	if resp.Error != "" {
		return "", "", fmt.Errorf("compile error: %s", resp.Error)
	}

	hash := sha256.Sum256([]byte(resp.CompiledJS))
	return resp.CompiledJS, hex.EncodeToString(hash[:]), nil
}

// Render renders a pre-compiled email template with the given props
// via NATS request/reply and returns the rendered HTML and plain text.
func (r *EmailRenderer) Render(ctx context.Context, projectID uuid.UUID, compiledJS string, props map[string]any) (*schemas.RenderEmailResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	reply, err := r.caller.Call(ctx, schemas.EmailRender(projectID), schemas.RenderEmail{
		CompiledJS: compiledJS,
		Props:      props,
	})
	if err != nil {
		return nil, fmt.Errorf("render email: %w", err)
	}

	var resp schemas.RenderEmailResponse
	if err := json.Unmarshal(reply, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal render response: %w", err)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("render error: %s", resp.Error)
	}

	return &resp, nil
}
