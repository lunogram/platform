package v1

import (
	"embed"
	"fmt"
	"io/fs"
	nethttp "net/http"

	"github.com/cloudproud/graceful"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	"github.com/lunogram/platform/internal/actions"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/http"
	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/http/console"
	clientv1 "github.com/lunogram/platform/internal/http/controllers/v1/client"
	clientoapi "github.com/lunogram/platform/internal/http/controllers/v1/client/oapi"
	managementv1 "github.com/lunogram/platform/internal/http/controllers/v1/management"
	mgmtoapi "github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/scalar"
	"github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/storage"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

//go:embed client/static
var staticFiles embed.FS

// NewServer constructs a unified HTTP server combining both management and client
// API endpoints. Management endpoints use JWT+API Key auth, while client endpoints
// use API Key only authentication.
func NewServer(ctx graceful.Context, logger *zap.Logger, cfg config.Node, db *store.Connections, storage storage.Storage, jet jetstream.JetStream, pub pubsub.Publisher, req pubsub.Caller, registry *providers.Registry, actionRegistry *actions.Registry, rbacEngine *rbac.Engine) (*http.Server, error) {
	mgmtStores := management.NewState(db.Management)
	usersStore := subjects.NewState(db.Subjects)

	// Load OpenAPI specs
	mgmtSpec, err := mgmtoapi.Spec()
	if err != nil {
		return nil, fmt.Errorf("failed to load management OpenAPI spec: %w", err)
	}

	clientSpec, err := clientoapi.Spec()
	if err != nil {
		return nil, fmt.Errorf("failed to load client OpenAPI spec: %w", err)
	}

	// Create management controller
	mgmtController, err := managementv1.NewController(logger, db.Management, db.Subjects, db.Journey, cfg, storage, pub, req, jet, registry, actionRegistry, rbacEngine)
	if err != nil {
		return nil, fmt.Errorf("failed to create management controller: %w", err)
	}

	// Create client controller
	clientController, err := clientv1.NewController(logger, db.Management, db.Subjects, mgmtStores, usersStore, pub, rbacEngine)
	if err != nil {
		return nil, fmt.Errorf("failed to create client controller: %w", err)
	}

	router := chi.NewRouter()
	router.Use(http.Logger(logger))

	// Serve unified API documentation with both specs
	router.Use(apiDocsMiddleware())

	// Mount management routes with JWT+API Key auth
	mgmtoapi.HandlerWithOptions(mgmtController, mgmtoapi.ChiServerOptions{
		BaseRouter: router,
		Middlewares: []mgmtoapi.MiddlewareFunc{mgmtoapi.Validator(mgmtSpec, openapi3filter.Options{
			AuthenticationFunc: auth.Middleware(
				auth.WithJWT(cfg.Auth, mgmtStores),
				auth.WithKey(mgmtStores),
			),
		})},
	})

	// Mount client routes with API Key only auth
	// Mount client routes with API Key only auth
	router.Group(func(r chi.Router) {
		r.Use(clientoapi.CORS())
		r.Options("/api/client/*", func(w nethttp.ResponseWriter, r *nethttp.Request) {})
		clientoapi.HandlerWithOptions(clientController, clientoapi.ChiServerOptions{
			BaseRouter: r,
			Middlewares: []clientoapi.MiddlewareFunc{
				clientoapi.Validator(clientSpec, openapi3filter.Options{
					AuthenticationFunc: auth.Middleware(
						auth.WithKey(mgmtStores),
					),
				}),
			},
		})
	})

	// Serve static assets - use sub-filesystem to strip the "client/static" prefix
	staticSubFS, err := fs.Sub(staticFiles, "client/static")
	if err != nil {
		return nil, fmt.Errorf("failed to create static sub-filesystem: %w", err)
	}
	router.Handle("/static/*", nethttp.StripPrefix("/static/", nethttp.FileServer(nethttp.FS(staticSubFS))))

	// Serve console (admin UI) as fallback
	consoleHandler, err := console.Handler()
	if err != nil {
		return nil, fmt.Errorf("failed to create console handler: %w", err)
	}
	router.Handle("/*", consoleHandler)

	return http.NewServer(logger, router, cfg.HTTP), nil
}

// apiDocsMiddleware serves the unified API documentation with both management and client specs.
func apiDocsMiddleware() func(next nethttp.Handler) nethttp.Handler {
	return func(next nethttp.Handler) nethttp.Handler {
		return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, req *nethttp.Request) {
			switch req.URL.Path {
			case "/api/openapi/management.yaml":
				w.Header().Set("Content-Type", "application/x-yaml")
				w.Write(mgmtoapi.OAPI) //nolint:errcheck
				return
			case "/api/openapi/client.yaml":
				w.Header().Set("Content-Type", "application/x-yaml")
				w.Write(clientoapi.OAPI) //nolint:errcheck
				return
			case "/api", "/api/":
				content, err := scalar.FS.ReadFile("index.html")
				if err != nil {
					nethttp.Error(w, "Internal Server Error", nethttp.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Write(content) //nolint:errcheck
				return
			}

			next.ServeHTTP(w, req)
		})
	}
}
