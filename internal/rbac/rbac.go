package rbac

import (
	"context"

	"github.com/google/uuid"
)

// ActorType distinguishes how the actor was authenticated.
type ActorType string

const (
	ActorAdmin   ActorType = "admin"
	ActorAPIKey  ActorType = "api_key"
	ActorEndUser ActorType = "end_user"
)

const (
	OrganizationMember string = "member"
	OrganizationAdmin  string = "admin"
	OrganizationOwner  string = "owner"
)

const (
	ProjectSupport string = "support"
	ProjectClient  string = "client"
	ProjectEditor  string = "editor"
	ProjectAdmin   string = "admin"
)

// Actor represents the authenticated entity making a request.
// It carries identity and scope information resolved at authentication time.
// Permission checks are delegated to the OpenFGA engine via [Engine.Allowed].
type Actor struct {
	Type           ActorType
	ID             string
	OrganizationID uuid.UUID
	ProjectID      uuid.UUID

	// SubjectID is the resolved internal Lunogram user (subject) id for
	// end-user actors ([ActorEndUser]). It is the zero UUID for admins and API
	// keys.
	//
	// Authorization is always decided in OpenFGA against the policy identity
	// ([Actor.UserKey]); SubjectID never participates in the permission check.
	// It exists so handlers can scope queries to the verified user's own rows
	// (e.g. WHERE user_id = SubjectID) once the policy has authorized the
	// resource and verb.
	SubjectID uuid.UUID

	// OwnData reports whether the actor is confined to its own records. It is
	// set only for verified end-user actors ([ActorEndUser]) whose auth method
	// carries the "own" subject scope; such actors are bound to their verified
	// subject regardless of any client-supplied identifier. A verified end user
	// configured for "all" data acts across subjects (like a backend key), so
	// OwnData is false. See [Actor.SubjectID].
	OwnData bool
}

// ActorOption configures optional fields on an [Actor].
type ActorOption func(*Actor)

// WithOrganizationID sets the organization scope on the actor.
func WithOrganizationID(id uuid.UUID) ActorOption {
	return func(a *Actor) {
		a.OrganizationID = id
	}
}

// WithProjectID sets the project scope on the actor.
func WithProjectID(id uuid.UUID) ActorOption {
	return func(a *Actor) {
		a.ProjectID = id
	}
}

// WithSubjectID sets the resolved internal user (subject) id on the actor. Use
// this for end-user actors so handlers can scope queries to the verified user's
// own rows. See [Actor.SubjectID].
func WithSubjectID(id uuid.UUID) ActorOption {
	return func(a *Actor) {
		a.SubjectID = id
	}
}

// WithOwnData confines the actor to its own records. Use this for verified
// end-user actors whose auth method has the "own" subject scope. See
// [Actor.OwnData].
func WithOwnData(own bool) ActorOption {
	return func(a *Actor) {
		a.OwnData = own
	}
}

// NewActor creates an Actor with the given identity. Use [ActorOption]
// functions to set the organization ID and project ID.
//
//	rbac.NewActor(rbac.ActorAdmin, id,
//	    rbac.WithOrganizationID(orgID),
//	)
//
//	rbac.NewActor(rbac.ActorAPIKey, id,
//	    rbac.WithOrganizationID(orgID),
//	    rbac.WithProjectID(projectID),
//	)
func NewActor(actorType ActorType, id string, opts ...ActorOption) *Actor {
	a := &Actor{
		Type: actorType,
		ID:   id,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// UserKey returns the OpenFGA user key for this actor (e.g. "user:<id>").
func (a *Actor) UserKey() string {
	return "user:" + a.ID
}

// Permission represents a CRUD action checked against a [Scope].
// These map directly to OpenFGA relation names on the resource types.
type Permission string

const (
	Read   Permission = "read"
	Create Permission = "create"
	Update Permission = "update"
	Delete Permission = "delete"
)

// Scope identifies the OpenFGA object a permission is checked against.
// Use [OrganizationScope], [ProjectScope], or [ProjectResourceScope] to
// construct one.
//
// The underlying string is an OpenFGA object key such as
// "organization:<uuid>", "project:<uuid>", or "users:<project-uuid>".
type Scope = string

// OrganizationScope returns a scope that targets the given organization.
// Use this for organization-level permission checks (e.g. managing admins,
// projects, API keys, or the organization itself).
//
//	engine.Allowed(ctx, rbac.Read, rbac.OrganizationScope(actor.OrganizationID))
func OrganizationScope(organizationID uuid.UUID) Scope {
	return "organization:" + organizationID.String()
}

// ProjectScope returns a scope that targets the given project.
// Use this for project-level permission checks (e.g. managing resources within a project).
//
//	engine.Allowed(ctx, rbac.Read, rbac.ProjectScope(projectID))
func ProjectScope(projectID uuid.UUID) Scope {
	return "project:" + projectID.String()
}

// ProjectResourceScope returns a scope that targets a specific resource type within a
// project. The resource name must match one of the resource type definitions in
// the authorization model (e.g. "users", "campaigns", "journeys").
//
//	engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("users", projectID))
//	engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("journeys", projectID))
func ProjectResourceScope(resource string, projectID uuid.UUID) Scope {
	return resource + ":" + projectID.String()
}

type actorContextKey struct{}

// WithActor stores the actor in the context.
func WithActor(ctx context.Context, actor *Actor) context.Context {
	return context.WithValue(ctx, actorContextKey{}, actor)
}

// FromContext retrieves the actor from the context. Returns nil if absent.
func FromContext(ctx context.Context) *Actor {
	actor, _ := ctx.Value(actorContextKey{}).(*Actor)
	return actor
}
