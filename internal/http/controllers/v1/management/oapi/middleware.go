package oapi

import (
	"errors"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/http/scalar"
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

func Scalar() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.URL.Path == "/openapi.yaml" {
				scalar.HandleOAPI(oapi).ServeHTTP(w, req)
				return
			}
			if req.URL.Path == "/" {
				http.FileServer(http.FS(scalar.FS)).ServeHTTP(w, req)
				return
			}

			next.ServeHTTP(w, req)
		})
	}
}
