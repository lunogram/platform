package v1

import (
	_ "embed"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lunogram/platform/services/nexus/oapi"
)

// Use set's up the v1 OpenAPI endpoints server handler within the given router.
func Use(router chi.Router, server oapi.ServerInterface, middleware ...oapi.MiddlewareFunc) http.Handler {
	return oapi.HandlerWithOptions(server, oapi.ChiServerOptions{
		BaseRouter:  router,
		Middlewares: middleware,
	})
}
