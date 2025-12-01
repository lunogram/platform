package rbac

import (
	"context"

	"github.com/google/uuid"
)

type contextKey string

const adminKey contextKey = "admin"

// Admin represents an authenticated admin user in the context
type Admin struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
}

// WithAdmin stores the admin object in the context
func WithAdmin(ctx context.Context, admin *Admin) context.Context {
	return context.WithValue(ctx, adminKey, admin)
}

// FromContext retrieves the admin object from the context.
// Returns nil if no admin is present.
func FromContext(ctx context.Context) *Admin {
	admin, _ := ctx.Value(adminKey).(*Admin)
	return admin
}
