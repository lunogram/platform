package auth

import (
	"errors"
	"net/http"

	"github.com/lunogram/platform/internal/http/problem"
)

// Require returns a plain HTTP middleware that runs the same [Handler] chain as
// [Middleware], for routes mounted outside the OpenAPI validator and therefore
// never reached by its AuthenticationFunc. Any route registered directly on the
// router — a reverse proxy, a hand-rolled handler — has to opt in through this
// middleware or it carries no credential check at all.
//
// The credential is read with [GetSession], so the same admin session cookie or
// bearer token works here as on the OpenAPI surfaces. On success the request is
// passed downstream with the authenticated context, carrying the resolved rbac
// actor. A rejected credential renders a 401 through writeProblem in the calling
// surface's problem shape; a handler that fails for any other reason (a database
// error while resolving the actor, say) renders a 500 rather than falling
// through to the next handler, so a transient failure cannot fail open.
//
// The response writer is handed to the next handler untouched. Do not wrap it
// here: these routes carry streaming responses whose upstream writer must keep
// its http.Flusher for the flush to reach the client.
func Require(writeProblem func(http.ResponseWriter, error), handlers ...Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, err := authenticate(r.Context(), r, handlers)
			if err != nil {
				if errors.Is(err, ErrUnauthorized) {
					writeProblem(w, problem.ErrUnauthorized())
					return
				}

				// An error that already carries a status is a considered answer
				// -- a refused origin, say -- and is passed through with its
				// description intact. Rendering it as a 500 would throw away the
				// only explanation the caller gets.
				if problem.HasStatus(err) {
					writeProblem(w, err)
					return
				}

				writeProblem(w, problem.ErrInternal())
				return
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
