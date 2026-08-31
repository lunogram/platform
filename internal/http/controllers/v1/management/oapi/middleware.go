package oapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/lunogram/platform/internal/http/problem"
	middleware "github.com/oapi-codegen/nethttp-middleware"
)

func Validator(spec *openapi3.T, options openapi3filter.Options) func(next http.Handler) http.Handler {
	return middleware.OapiRequestValidatorWithOptions(spec, &middleware.Options{
		Options:              options,
		DoNotValidateServers: true,
		ErrorHandlerWithOpts: func(_ context.Context, err error, w http.ResponseWriter, _ *http.Request, opts middleware.ErrorHandlerOpts) {
			WriteProblem(w, validationProblem(err, opts.StatusCode))
		},
	})
}

// validationProblem picks the problem a validator failure is answered with.
//
// An authentication handler that refused the request with a status of its own
// is answered with exactly that. The validator reports every security failure
// as a 401, which would otherwise flatten a deliberate 403 -- a refused origin,
// say -- into "your session has expired", and the console acts on that by
// sending the admin to the login page they just came from.
func validationProblem(err error, status int) error {
	var security *openapi3filter.SecurityRequirementsError
	if errors.As(err, &security) {
		for _, cause := range security.Errors {
			if problem.HasStatus(cause) {
				return cause
			}
		}
	}

	message := err.Error()
	return problem.WithStatus(problem.WithDescription(errors.New(message), "bad request", message), status)
}
