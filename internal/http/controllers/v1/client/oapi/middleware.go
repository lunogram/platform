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

func CORS() func(next http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}
