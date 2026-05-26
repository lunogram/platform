package rbac

import (
	"context"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/problem"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/openfga/openfga/pkg/logger"
	"github.com/openfga/openfga/pkg/server"
	"github.com/openfga/openfga/pkg/storage"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Engine wraps the embedded OpenFGA server to provide relationship-based
// access control. It owns the store and authorization model lifecycle.
type Engine struct {
	server       *server.Server
	datastore    storage.OpenFGADatastore
	storeID      string
	modelID      string
	modelChanged bool
}

// Tuple is a convenience type for batch read/write/delete operations.
type Tuple struct {
	User     string
	Relation string
	Object   string
}

// NewEngine initializes the embedded OpenFGA engine backed by the management
// database. It ensures the store and authorization model exist. The provided
// context is used for initialization; pass the application's graceful context
// so that startup is cancelled when a shutdown signal is received.
func NewEngine(ctx context.Context, config Config) (*Engine, error) {
	datastore, err := NewStore(config)
	if err != nil {
		return nil, fmt.Errorf("rbac: failed to initialize openfga datastore: %w", err)
	}

	s, err := server.NewServerWithOpts(
		server.WithDatastore(datastore),
		server.WithLogger(logger.NewNoopLogger()),
	)
	if err != nil {
		datastore.Close()
		return nil, fmt.Errorf("rbac: failed to create openfga server: %w", err)
	}

	e := &Engine{server: s, datastore: datastore}

	if err := e.ensureStore(ctx); err != nil {
		e.Close()
		return nil, err
	}
	if err := e.ensureModel(ctx); err != nil {
		e.Close()
		return nil, err
	}

	return e, nil
}

// Close shuts down the underlying OpenFGA server and closes the datastore,
// releasing all resources including database connection pools.
func (e *Engine) Close() {
	if e.server != nil {
		e.server.Close()
	}
	if e.datastore != nil {
		e.datastore.Close()
	}
}

// Allowed verifies that the authenticated actor in ctx holds the given
// permission on scope. The actor is read from the context via [FromContext].
//
// Use [OrganizationScope] for organization-level checks and
// [ProjectResourceScope] for project-resource-level checks.
//
//	engine.Allowed(ctx, rbac.Read, rbac.OrganizationScope(actor.OrganizationID))
//	engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("users", projectID))
//
// Returns nil when the actor is allowed, problem.ErrUnauthorized when no
// actor is present in ctx, or problem.ErrForbidden when the actor lacks the
// permission.
func (e *Engine) Allowed(ctx context.Context, permission Permission, scope Scope) error {
	actor := FromContext(ctx)
	if actor == nil {
		return problem.ErrUnauthorized()
	}

	allowed, err := e.Check(ctx, actor.UserKey(), string(permission), scope)
	if err != nil {
		return fmt.Errorf("rbac: permission check failed: %w", err)
	}

	if !allowed {
		return problem.ErrForbidden(problem.Describe("missing permission: " + string(permission)))
	}

	return nil
}

// AllowedProject verifies that the authenticated actor in ctx has a valid
// project scope and holds the given permission on the specified resource
// within that project.
//
// This combines actor extraction, project scope validation, and permission
// checking into a single call. It is the recommended way to authorize
// project-scoped API handlers.
//
//	projectID, err := engine.AllowedProject(ctx, "users", rbac.Create)
//	projectID, err := engine.AllowedProject(ctx, "inbox", rbac.Read)
//
// Returns the actor's project ID when authorized, or an error suitable for
// writing as an HTTP problem response.
func (e *Engine) AllowedProject(ctx context.Context, resource string, permission Permission) (uuid.UUID, error) {
	actor := FromContext(ctx)
	if actor == nil {
		return uuid.Nil, problem.ErrUnauthorized()
	}

	if actor.ProjectID == uuid.Nil {
		return uuid.Nil, problem.ErrUnauthorized(problem.Describe("project scope is required"))
	}

	allowed, err := e.Check(ctx, actor.UserKey(), string(permission), ProjectResourceScope(resource, actor.ProjectID))
	if err != nil {
		return uuid.Nil, fmt.Errorf("rbac: permission check failed: %w", err)
	}

	if !allowed {
		return uuid.Nil, problem.ErrForbidden(problem.Describe("missing permission: " + string(permission)))
	}

	return actor.ProjectID, nil
}

// Check returns true when the given user has the specified relation on the
// object. This is the low-level OpenFGA check; prefer [Engine.Allowed] for
// permission checks that read the actor from context.
//
//	user:     "user:<uuid>"
//	relation: "read", "create", "admin", etc.
//	object:   "organization:<uuid>", "users:<project-uuid>", etc.
func (e *Engine) Check(ctx context.Context, user, relation, object string) (bool, error) {
	resp, err := e.server.Check(ctx, &openfgav1.CheckRequest{
		StoreId:              e.storeID,
		AuthorizationModelId: e.modelID,
		TupleKey: &openfgav1.CheckRequestTupleKey{
			User:     user,
			Relation: relation,
			Object:   object,
		},
	})
	if err != nil {
		return false, fmt.Errorf("openfga check failed: %w", err)
	}
	return resp.Allowed, nil
}

// WriteTuple adds a single relationship tuple.
func (e *Engine) WriteTuple(ctx context.Context, user, relation, object string) error {
	_, err := e.server.Write(ctx, &openfgav1.WriteRequest{
		StoreId:              e.storeID,
		AuthorizationModelId: e.modelID,
		Writes: &openfgav1.WriteRequestWrites{
			TupleKeys: []*openfgav1.TupleKey{
				{User: user, Relation: relation, Object: object},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("openfga write failed: %w", err)
	}
	return nil
}

// DeleteTuple removes a single relationship tuple.
func (e *Engine) DeleteTuple(ctx context.Context, user, relation, object string) error {
	_, err := e.server.Write(ctx, &openfgav1.WriteRequest{
		StoreId:              e.storeID,
		AuthorizationModelId: e.modelID,
		Deletes: &openfgav1.WriteRequestDeletes{
			TupleKeys: []*openfgav1.TupleKeyWithoutCondition{
				{User: user, Relation: relation, Object: object},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("openfga delete failed: %w", err)
	}
	return nil
}

// WriteTuples writes multiple relationship tuples in a single request.
func (e *Engine) WriteTuples(ctx context.Context, tuples []Tuple) error {
	if len(tuples) == 0 {
		return nil
	}
	keys := make([]*openfgav1.TupleKey, len(tuples))
	for i, t := range tuples {
		keys[i] = &openfgav1.TupleKey{User: t.User, Relation: t.Relation, Object: t.Object}
	}
	_, err := e.server.Write(ctx, &openfgav1.WriteRequest{
		StoreId:              e.storeID,
		AuthorizationModelId: e.modelID,
		Writes:               &openfgav1.WriteRequestWrites{TupleKeys: keys},
	})
	if err != nil {
		return fmt.Errorf("openfga batch write failed: %w", err)
	}
	return nil
}

// DeleteTuples removes multiple relationship tuples in a single request.
func (e *Engine) DeleteTuples(ctx context.Context, tuples []Tuple) error {
	if len(tuples) == 0 {
		return nil
	}
	keys := make([]*openfgav1.TupleKeyWithoutCondition, len(tuples))
	for i, t := range tuples {
		keys[i] = &openfgav1.TupleKeyWithoutCondition{User: t.User, Relation: t.Relation, Object: t.Object}
	}
	_, err := e.server.Write(ctx, &openfgav1.WriteRequest{
		StoreId:              e.storeID,
		AuthorizationModelId: e.modelID,
		Deletes:              &openfgav1.WriteRequestDeletes{TupleKeys: keys},
	})
	if err != nil {
		return fmt.Errorf("openfga batch delete failed: %w", err)
	}
	return nil
}

func (e *Engine) ensureStore(ctx context.Context) error {
	stores, err := e.server.ListStores(ctx, &openfgav1.ListStoresRequest{})
	if err != nil {
		return fmt.Errorf("rbac: failed to list stores: %w", err)
	}

	const storeName = "lunogram"
	for _, store := range stores.Stores {
		if store.Name == storeName {
			e.storeID = store.Id
			return nil
		}
	}

	resp, err := e.server.CreateStore(ctx, &openfgav1.CreateStoreRequest{Name: storeName})
	if err != nil {
		return fmt.Errorf("rbac: failed to create store: %w", err)
	}
	e.storeID = resp.Id
	return nil
}

// ensureModel reads the latest authorization model from the store and compares
// it to the desired model. If the models match, it reuses the existing model
// ID. Otherwise it writes a new model version.
func (e *Engine) ensureModel(ctx context.Context) error {
	desired := Model()

	// Try to read the latest model from the store.
	latest, err := e.server.ReadAuthorizationModels(ctx, &openfgav1.ReadAuthorizationModelsRequest{
		StoreId:  e.storeID,
		PageSize: wrapperspb.Int32(1),
	})
	if err == nil && len(latest.GetAuthorizationModels()) > 0 {
		model := latest.GetAuthorizationModels()[0]
		if modelsEqual(model.GetTypeDefinitions(), desired) {
			e.modelID = model.GetId()
			return nil
		}
	}

	resp, err := e.server.WriteAuthorizationModel(ctx, &openfgav1.WriteAuthorizationModelRequest{
		StoreId:         e.storeID,
		SchemaVersion:   "1.1",
		TypeDefinitions: desired,
	})
	if err != nil {
		return fmt.Errorf("rbac: failed to write authorization model: %w", err)
	}
	e.modelID = resp.AuthorizationModelId
	e.modelChanged = true
	return nil
}

// ModelChanged returns true if the authorization model was updated during
// engine initialization (i.e. a new type was added or an existing one
// modified). Callers can use this to trigger backfill operations such as
// writing resource tuples for existing projects.
func (e *Engine) ModelChanged() bool {
	return e.modelChanged
}

// modelsEqual compares two slices of type definitions using protobuf
// equality. Slices are sorted by type name before comparison so that
// ordering differences do not cause false negatives.
func modelsEqual(a, b []*openfgav1.TypeDefinition) bool {
	if len(a) != len(b) {
		return false
	}
	sort.Slice(a, func(i, j int) bool { return a[i].GetType() < a[j].GetType() })
	sort.Slice(b, func(i, j int) bool { return b[i].GetType() < b[j].GetType() })
	for i := range a {
		if !proto.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}
