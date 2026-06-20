package v1

import (
	"context"

	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/subjects"
)

// boundUserIdentifiers returns the external-ID identifiers a user-facing request
// must be scoped to.
//
// For an own-data actor (a verified end user — trusted-issuer JWT or Lunogram
// session — whose auth method has the "own" subject scope) the request is bound
// to the verified subject, ignoring any client-supplied identifier, so it can
// only ever read or write its own data. Backend callers (API keys) and verified
// users configured for "all" data use the supplied identifiers as-is, which is
// what lets a trusted integration act across users.
func boundUserIdentifiers(ctx context.Context, supplied []subjects.ExternalIDParam) []subjects.ExternalIDParam {
	if actor := rbac.FromContext(ctx); actor != nil && actor.Scope == rbac.DataScopeOwn {
		return []subjects.ExternalIDParam{{Source: actor.SubjectSource, ExternalID: actor.Subject}}
	}
	return supplied
}

// isOwnDataScoped reports whether the request is confined to its own records: a
// verified end user whose auth method carries the "own" subject scope.
func isOwnDataScoped(ctx context.Context) bool {
	actor := rbac.FromContext(ctx)
	return actor != nil && actor.Scope == rbac.DataScopeOwn
}

// requireCrossSubjectAccess returns a problem when an own-data actor targets an
// endpoint that acts beyond a single verified subject (e.g. anything
// organization-scoped). "Own data" has no meaning for an organization, so rather
// than silently letting a confined end user act across a whole organization
// these requests fail closed.
func requireCrossSubjectAccess(ctx context.Context) error {
	if isOwnDataScoped(ctx) {
		return problem.ErrForbidden(problem.Describe("end users may only access their own data"))
	}
	return nil
}
