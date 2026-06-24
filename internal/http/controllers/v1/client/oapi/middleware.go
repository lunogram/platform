package oapi

import (
	"errors"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/cors"
	"github.com/lunogram/platform/internal/http/problem"
	middleware "github.com/oapi-codegen/nethttp-middleware"
)

func Validator(spec *openapi3.T, options openapi3filter.Options) func(next http.Handler) http.Handler {
	return middleware.OapiRequestValidatorWithOptions(spec, &middleware.Options{
		Options:              options,
		DoNotValidateServers: true,
		ErrorHandler: func(w http.ResponseWriter, message string, statusCode int) {
			err := problem.WithStatus(problem.WithDescription(errors.New(message), "bad request", message), statusCode)
			WriteProblem(w, err)
		},
	})
}

// CORS configures cross-origin access for the client API. The client API is
// strictly bearer-authenticated (no cookies), so credentials are disabled and a
// wildcard origin is safe: a bearer token is never sent ambiently by the
// browser, so a wildcard cannot be abused the way it can with cookie auth.
//
// API keys are private (backend-only) and rejected on browser-originated client
// requests, so the browser-facing client API is reached via a trusted issuer or
// a short-lived session; the wildcard is bounded by those credentials being
// write-only, event-allow-listed and own-data scoped, not by origin.
//
// The per-request rate limiter lives in internal/http (RateLimit) so it is
// shared with the management API and a key gets one budget across both.
func CORS() func(next http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	})
}
