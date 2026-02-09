package rbac

import (
	"context"

	"github.com/google/uuid"
)

type contextKey string

const scopeKey contextKey = "admin"

// Scope represents an authenticated user or API key in the context.
// It is used for both the management API (JWT authentication) and the client API (API key authentication).
type Scope struct {
	OrganizationID uuid.UUID
	ProjectID      uuid.UUID
}

// WithScope stores the admin object in the context
func WithScope(ctx context.Context, admiscope *Scope) context.Context {
	return context.WithValue(ctx, scopeKey, admiscope)
}

// FromContext retrieves the admin object from the context.
// Returns nil if no admin is present.
func FromContext(ctx context.Context) *Scope {
	admin, _ := ctx.Value(scopeKey).(*Scope)
	return admin
}
