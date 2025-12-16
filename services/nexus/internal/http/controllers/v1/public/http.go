package v1

import (
	_ "embed"
	"fmt"
	nethttp "net/http"
	"net/url"

	"github.com/cloudproud/graceful"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/pkg/http"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/http/auth"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/public/oapi"
	"github.com/lunogram/platform/services/nexus/internal/storage"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"go.uber.org/zap"
)

// NewServer constructs a new HTTP server and it's routes. The returned server
// could be used to listen and serve incoming requests on the given address.
func NewServer(ctx graceful.Context, logger *zap.Logger, config config.Service, db *sqlx.DB, storage storage.Storage) (*http.Server, error) {
	spec, err := oapi.Spec()
	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI spec: %w", err)
	}

	platform, err := url.Parse(config.PlatformURL)
	if err != nil {
		return nil, fmt.Errorf("invalid platform URL: %w", err)
	}

	platformProxy := http.ReverseProxy(platform)

	stores := store.NewStores(db)

	options := openapi3filter.Options{
		AuthenticationFunc: auth.Middleware(auth.WithJWT(config, stores), auth.WithKey(stores)),
	}

	controller := NewController(logger, db, platformProxy)

	router := chi.NewRouter()
	router.Use(http.Logger(logger))

	router.Use(oapi.Scalar())

	// Add subscription preference routes (these are not in OpenAPI spec as they return HTML)
	router.Get("/unsubscribe/email", controller.UnsubscribeEmail)
	router.Get("/preferences/{userID}", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		userID := chi.URLParam(r, "userID")
		controller.GetPreferences(w, r, userID)
	})
	router.Post("/preferences/{userID}", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		userID := chi.URLParam(r, "userID")
		controller.UpdatePreferences(w, r, userID)
	})
	router.Get("/static/*", controller.ServeStaticFiles)

	oapi.HandlerWithOptions(controller, oapi.ChiServerOptions{
		BaseRouter:  router,
		Middlewares: []oapi.MiddlewareFunc{oapi.Validator(spec, options)},
	})

	// NOTE: during the migration we proxy all unknown requests to the platform service.
	router.Handle("/*", platformProxy)
	return http.NewServer(logger, router, config.HTTP), nil
}
