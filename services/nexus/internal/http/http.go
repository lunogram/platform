package http

import (
	_ "embed"
	"fmt"
	"net/url"

	"github.com/cloudproud/graceful"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/pkg/http"
	"github.com/lunogram/platform/services/nexus/internal/config"
	v1 "github.com/lunogram/platform/services/nexus/internal/http/controllers/v1"
	"github.com/lunogram/platform/services/nexus/oapi"
	"go.uber.org/zap"
)

// NewServer constructs a new HTTP server and it's routes. The returned server
// could be used to listen and serve incoming requests on the given address.
func NewServer(ctx graceful.Context, logger *zap.Logger, config config.Service, db *sqlx.DB) (*http.Server, error) {
	spec, err := oapi.Spec()
	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI spec: %w", err)
	}

	options := openapi3filter.Options{
		AuthenticationFunc: Auth(config),
	}

	router := chi.NewRouter()
	router.Use(Logger(logger))

	router.Use(oapi.Scalar())
	v1.Use(router, v1.NewController(logger, db), oapi.Validator(spec, options))

	platform, err := url.Parse(config.PlatformURL)
	if err != nil {
		return nil, fmt.Errorf("invalid platform URL: %w", err)
	}

	// NOTE: during the migration we proxy all unknown requests to the platform service.
	router.Handle("/*", http.ReverseProxy(platform))

	return http.NewServer(logger, router, config.HTTP), nil
}
