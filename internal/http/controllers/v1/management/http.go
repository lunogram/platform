package v1

import (
	_ "embed"
	"fmt"

	"github.com/cloudproud/graceful"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/http"
	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/http/console"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/storage"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

// NewServer constructs a new HTTP server and it's routes. The returned server
// could be used to listen and serve incoming requests on the given address.
func NewServer(ctx graceful.Context, logger *zap.Logger, config config.Node, db *sqlx.DB, storage storage.Storage, pub pubsub.Publisher, registry *providers.Registry) (*http.Server, error) {
	spec, err := oapi.Spec()
	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI spec: %w", err)
	}

	stores := management.NewState(db)

	controller, err := NewController(logger, db, config, storage, pub, registry)
	if err != nil {
		return nil, fmt.Errorf("failed to create controller: %w", err)
	}

	options := openapi3filter.Options{
		AuthenticationFunc: auth.Middleware(
			auth.WithJWT(config.Auth, stores),
			auth.WithKey(stores),
		),
	}

	router := chi.NewRouter()
	router.Use(http.Logger(logger))

	router.Use(oapi.Scalar())

	oapi.HandlerWithOptions(controller, oapi.ChiServerOptions{
		BaseRouter:  router,
		Middlewares: []oapi.MiddlewareFunc{oapi.Validator(spec, options)},
	})

	consoleHandler, err := console.Handler()
	if err != nil {
		return nil, fmt.Errorf("failed to create console handler: %w", err)
	}
	router.Handle("/*", consoleHandler)

	return http.NewServer(logger, router, config.HTTP), nil
}
