package auth

import (
	"context"

	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/subjects"
)

// Request-scoping helpers derived from the authenticated rbac actor. They live
// here (rather than in a single controller package) so any surface can apply the
// same own-data rules.

// BoundUserIdentifiers returns the external-ID identifiers a user-facing request
// must be scoped to.
//
// For an own-data actor (a verified end user — trusted-issuer JWT or Lunogram
// session — whose auth method has the "own" subject scope) the request is bound
// to the verified subject, ignoring any client-supplied identifier, so it can
// only ever read or write its own data. Backend callers (API keys) and verified
// users configured for "all" data use the supplied identifiers as-is, which is
// what lets a trusted integration act across users.
func BoundUserIdentifiers(ctx context.Context, supplied []subjects.ExternalIDParam) []subjects.ExternalIDParam {
	if actor := rbac.FromContext(ctx); actor != nil && actor.Scope == rbac.DataScopeOwn {
		return []subjects.ExternalIDParam{{Source: actor.SubjectSource, ExternalID: actor.Subject}}
	}
	return supplied
}

// OwnDataScoped reports whether the request is confined to its own records: a
// verified end user whose auth method carries the "own" subject scope.
func OwnDataScoped(ctx context.Context) bool {
	actor := rbac.FromContext(ctx)
	return actor != nil && actor.Scope == rbac.DataScopeOwn
}

// RequireCrossSubjectAccess returns a problem when an own-data actor targets an
// endpoint that acts beyond a single verified subject (e.g. anything
// organization-scoped). "Own data" has no meaning for an organization, so rather
// than silently letting a confined end user act across a whole organization
// these requests fail closed.
func RequireCrossSubjectAccess(ctx context.Context) error {
	if OwnDataScoped(ctx) {
		return problem.ErrForbidden(problem.Describe("end users may only access their own data"))
	}
	return nil
}

// IsClientContext reports whether the request comes from an untrusted client
// context — a verified end user (trusted issuer or session) — as opposed to a
// trusted backend API key. API keys are private/backend-only, so they are never
// a client context. The event allow-list applies only to client contexts.
func IsClientContext(ctx context.Context) bool {
	actor := rbac.FromContext(ctx)
	return actor != nil && actor.Type == rbac.ActorEndUser
}
