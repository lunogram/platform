package v1

import (
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/rbac"
)

// adminActorID returns the admin id of an admin actor, and an error otherwise.
//
// [rbac.Actor.ID] means three different things depending on [rbac.Actor.Type]:
// an admin UUID, an API-key id, or an auth-method UUID. Handlers used to assert
// the admin flavour by parsing the id as a UUID -- but the auth-method id IS a
// UUID, so the parse succeeded for an API-key actor and the request continued
// with a key id where an admin id was expected, producing a 500 deep in a query
// instead of the 403 it deserved. The type is the only thing that actually
// answers "is this an admin?", so it is what gets checked.
func adminActorID(actor *rbac.Actor) (uuid.UUID, error) {
	if actor == nil || actor.ID == "" {
		return uuid.Nil, problem.ErrUnauthorized()
	}
	if actor.Type != rbac.ActorAdmin {
		return uuid.Nil, problem.ErrForbidden(problem.Describe("this endpoint is only available to admins"))
	}
	id, err := uuid.Parse(actor.ID)
	if err != nil {
		return uuid.Nil, problem.ErrUnauthorized()
	}
	return id, nil
}

// personalAdminID is the admin id to personalise a response with -- the caller's
// project role, their own project list -- or nil when the caller is not an admin.
//
// It is the counterpart to [adminActorID] for endpoints an API key may legitimately
// call: those callers get the unpersonalised view rather than a 403. Passing them
// through as an admin id would silently query project_admins for an auth-method
// id, which matches nothing, so the caller would be told they have no role rather
// than that the question does not apply to them.
func personalAdminID(actor *rbac.Actor) *uuid.UUID {
	if actor == nil || actor.Type != rbac.ActorAdmin {
		return nil
	}
	id, err := uuid.Parse(actor.ID)
	if err != nil {
		return nil
	}
	return &id
}
