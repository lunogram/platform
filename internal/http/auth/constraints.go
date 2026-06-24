package auth

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
)

// CreateConstraints enforces an auth method's per-grant create constraints for
// the current request. It is a thin behavior over the management store — there
// is only ever one implementation, so it takes the concrete store rather than an
// interface.
type CreateConstraints struct {
	store *management.State
}

// NewCreateConstraints builds the enforcer over the given management store.
func NewCreateConstraints(store *management.State) CreateConstraints {
	return CreateConstraints{store: store}
}

// Enforce rejects any name the request's auth method may not create for
// resource. The constraint travels with the credential — its method id is the
// request actor — so it applies uniformly to every auth-method type (api key,
// trusted issuer, or session). A method with no constraint for resource is
// unrestricted. It fails closed: a store failure or an actor that is not an auth
// method yields an internal error rather than allowing the write.
func (c CreateConstraints) Enforce(ctx context.Context, resource rbac.Resource, names []string) error {
	actor := rbac.FromContext(ctx)
	if actor == nil {
		return problem.ErrUnauthorized()
	}
	methodID, err := uuid.Parse(actor.ID)
	if err != nil {
		return problem.ErrInternal(problem.Describe("actor is not an auth method"))
	}

	allowed, restricted, err := c.store.CreateGrantInstances(ctx, methodID, string(resource))
	if err != nil {
		return problem.ErrInternal()
	}
	if !restricted {
		return nil
	}

	for _, name := range names {
		if !slices.Contains(allowed, name) {
			return problem.ErrForbidden(problem.Describe(
				fmt.Sprintf("auth method may not create %s %q", resource, name)))
		}
	}
	return nil
}
