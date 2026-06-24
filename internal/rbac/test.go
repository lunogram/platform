package rbac

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/openfga/openfga/pkg/logger"
	"github.com/openfga/openfga/pkg/server"
	"github.com/openfga/openfga/pkg/storage/memory"
)

// NewTestEngine creates an in-memory RBAC engine suitable for unit tests.
// It initializes the OpenFGA server with the in-memory storage backend,
// creates a store, and writes the authorization model.
//
// The returned engine must not be used concurrently across test cases that
// share state; prefer calling this once per test.
func NewTestEngine(t *testing.T) *Engine {
	t.Helper()

	ds := memory.New()
	t.Cleanup(ds.Close)

	s, err := server.NewServerWithOpts(
		server.WithDatastore(ds),
		server.WithLogger(logger.NewNoopLogger()),
	)
	if err != nil {
		t.Fatalf("rbac: failed to create in-memory openfga server: %v", err)
	}
	t.Cleanup(s.Close)

	e := &Engine{server: s, datastore: ds}

	ctx := context.Background()
	if err := e.ensureStore(ctx); err != nil {
		t.Fatalf("rbac: failed to ensure store: %v", err)
	}
	if err := e.ensureModel(ctx); err != nil {
		t.Fatalf("rbac: failed to ensure model: %v", err)
	}

	return e
}

// TestSetup creates a fresh in-memory RBAC engine and a context with the given
// actor, then writes the relationship tuples needed so that the actor has the
// specified org and project roles.
//
// orgRole should be one of "member", "admin", or "owner" (or empty to skip).
// projectRole should be one of "support", "client", "editor", or "admin"
// (or empty to skip).
//
// When a projectRole is provided and the actor has a non-nil ProjectID the
// following tuples are written:
//
//   - user:<id>  → <orgRole>  → organization:<org-id>
//   - user:<id>  → <projectRole> → project:<project-id>
//   - organization:<org-id> → organization → project:<project-id>
//   - project:<project-id> → project → <resource>:<project-id>  (for every resource type)
//
// The resource→project tuples are what allow CRUD permission checks on
// resource scopes (e.g. "users:<project-id>") to resolve through the parent
// project's role hierarchy.
//
// The returned engine should be injected as a dependency into the controller
// under test, while the context (which carries the actor) should be used for
// the request.
//
// Example:
//
//	engine, ctx := rbac.TestSetup(t, context.Background(), actor, "member", "admin")
//	// engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("users", projectID))
func TestSetup(t *testing.T, parent context.Context, actor *Actor, orgRole, projectRole string) (*Engine, context.Context) {
	t.Helper()

	engine := NewTestEngine(t)
	ctx := WithActor(parent, actor)

	user := actor.UserKey()
	orgObject := OrganizationScope(actor.OrganizationID)
	bg := context.Background()

	if orgRole != "" {
		if err := engine.WriteTuple(bg, user, orgRole, orgObject); err != nil {
			t.Fatalf("rbac: failed to write org role tuple (%s, %s, %s): %v", user, orgRole, orgObject, err)
		}
	}

	if projectRole != "" && actor.ProjectID != uuid.Nil {
		projectObject := ProjectScope(actor.ProjectID)

		// Link project to its parent organization so that tuple-to-userset
		// rewrites (org owner/admin → project admin) work.
		if err := engine.WriteTuple(bg, orgObject, "organization", projectObject); err != nil {
			t.Fatalf("rbac: failed to write project→org tuple: %v", err)
		}

		// Assign the project role to the user.
		if err := engine.WriteTuple(bg, user, projectRole, projectObject); err != nil {
			t.Fatalf("rbac: failed to write project role tuple (%s, %s, %s): %v", user, projectRole, projectObject, err)
		}

		// Link every resource type to this project so that CRUD checks on
		// ProjectResourceScope("resource", projectID) resolve through the project roles.
		for _, resource := range Resources() {
			resourceObject := resource + ":" + actor.ProjectID.String()
			if err := engine.WriteTuple(bg, projectObject, "project", resourceObject); err != nil {
				t.Fatalf("rbac: failed to write resource→project tuple (%s, project, %s): %v", projectObject, resourceObject, err)
			}
		}
	}

	return engine, ctx
}

// TestSetupWithTuples creates a fresh in-memory RBAC engine and a context with
// the given actor, then writes arbitrary relationship tuples. This is useful
// when the caller needs fine-grained control over which tuples exist.
//
// The returned engine should be injected as a dependency into the controller
// under test.
func TestSetupWithTuples(t *testing.T, parent context.Context, actor *Actor, tuples []Tuple) (*Engine, context.Context) {
	t.Helper()

	engine := NewTestEngine(t)
	ctx := WithActor(parent, actor)

	if len(tuples) > 0 {
		if err := engine.WriteTuples(context.Background(), tuples); err != nil {
			t.Fatalf("rbac: failed to write tuples: %v", err)
		}
	}

	return engine, ctx
}
