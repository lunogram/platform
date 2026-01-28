package v1

import (
	"embed"
	"fmt"
	"io/fs"
	nethttp "net/http"

	"github.com/cloudproud/graceful"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/http"
	"github.com/lunogram/platform/services/nexus/internal/http/auth"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/public/oapi"
	"github.com/lunogram/platform/services/nexus/internal/pubsub"
	"github.com/lunogram/platform/services/nexus/internal/storage"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"go.uber.org/zap"
)

//go:embed static
var staticFiles embed.FS

// NewServer constructs a new HTTP server and its routes. The returned server
// could be used to listen and serve incoming requests on the given address.
func NewServer(ctx graceful.Context, logger *zap.Logger, config config.Node, db *sqlx.DB, storage storage.Storage, pub pubsub.Publisher) (*http.Server, error) {
	spec, err := oapi.Spec()
	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI spec: %w", err)
	}

	stores := store.NewState(db)

	options := openapi3filter.Options{
		AuthenticationFunc: auth.Middleware(auth.WithJWT(config.Auth, stores), auth.WithKey(stores)),
	}

	router := chi.NewRouter()
	router.Use(http.Logger(logger))

	router.Use(oapi.Scalar())

	// Serve static files for CSS - use sub-filesystem to strip the "static" prefix
	staticSubFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, fmt.Errorf("failed to create static sub-filesystem: %w", err)
	}
	router.Handle("/static/*", nethttp.StripPrefix("/static/", nethttp.FileServer(nethttp.FS(staticSubFS))))

	controller, err := NewController(logger, db, pub)
	if err != nil {
		return nil, fmt.Errorf("failed to create controller: %w", err)
	}

	oapi.HandlerWithOptions(controller, oapi.ChiServerOptions{
		BaseRouter:  router,
		Middlewares: []oapi.MiddlewareFunc{oapi.Validator(spec, options)},
	})

	return http.NewServer(logger, router, config.HTTP), nil
}
