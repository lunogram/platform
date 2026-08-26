package v1

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"mime"
	nethttp "net/http"
	"path/filepath"
	"time"

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
	"github.com/lunogram/platform/internal/jwks"
	"github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/ratelimit"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/storage"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/nats-io/nats.go/jetstream"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

//go:embed client/static
var staticFiles embed.FS

// NewServer constructs a unified HTTP server combining both management and client
// API endpoints. Management endpoints authenticate with a Lunogram console
// session or an API key; client endpoints use API keys, client sessions, or a
// trusted issuer.
func NewServer(ctx graceful.Context, logger *zap.Logger, cfg config.Node, db *store.Connections, storageDriver storage.Storage, jet jetstream.JetStream, pub pubsub.Publisher, req pubsub.Caller, registry *providers.Registry, actionRegistry *actions.Registry, rbacEngine *rbac.Engine, limiter *ratelimit.Limiter, jwksCache *jwks.Cache, rdb *goredis.Client) (*http.Server, error) {
	mgmtStores := management.NewState(db.Management, management.WithRedis(rdb, cfg.Redis.KeyPrefix))
	usersStore := subjects.NewState(db.Subjects, logger)

	// Load OpenAPI specs
	mgmtSpec, err := mgmtoapi.Spec()
	if err != nil {
		return nil, fmt.Errorf("failed to load management OpenAPI spec: %w", err)
	}

	clientSpec, err := clientoapi.Spec()
	if err != nil {
		return nil, fmt.Errorf("failed to load client OpenAPI spec: %w", err)
	}

	// Create URL resolver for public document URLs
	urlResolver := storage.NewURLResolver(cfg.Storage.BaseURL)

	// Console session signer (ES256). Its key is separate from the client
	// session signer's, so a console token and a client token are separated by
	// cryptography rather than by claim shape.
	consoleSigner, err := auth.NewConsoleSigner(cfg.Auth.Console)
	if err != nil {
		return nil, fmt.Errorf("failed to build console session signer: %w", err)
	}
	if cfg.Auth.Driver != "" && consoleSigner == nil {
		// Fail hard rather than generating a key: an ephemeral key logs every
		// admin out on restart and breaks a multi-replica deployment silently,
		// since replicas cannot verify each other's tokens.
		return nil, auth.ErrConsoleSigningKeyMissing
	}

	// Create management controller
	mgmtController, err := managementv1.NewController(logger, db.Management, db.Subjects, db.Journey, cfg, storageDriver, urlResolver, pub, req, jet, registry, actionRegistry, rbacEngine, rdb, consoleSigner)
	if err != nil {
		return nil, fmt.Errorf("failed to create management controller: %w", err)
	}

	// Client session signer (ES256). Nil when no signing key is configured, which
	// disables session minting and verification.
	sessionSigner, err := auth.NewSessionSigner(cfg.Auth.SessionSigningKey, cfg.Auth.SessionIssuer)
	if err != nil {
		return nil, fmt.Errorf("failed to build session signer: %w", err)
	}

	// Create client controller
	clientController, err := clientv1.NewController(logger, db.Management, db.Subjects, mgmtStores, usersStore, pub, rbacEngine, sessionSigner)
	if err != nil {
		return nil, fmt.Errorf("failed to create client controller: %w", err)
	}

	router := chi.NewRouter()
	router.Use(http.Logger(logger))

	// Serve unified API documentation with both specs
	router.Use(apiDocsMiddleware())

	// Mount management routes with console-session + API-key auth. The legacy
	// upgrade runs OUTSIDE the OpenAPI validator because it needs the
	// ResponseWriter to set the console cookie, which the AuthenticationFunc
	// signature does not provide.
	mgmtoapi.HandlerWithOptions(mgmtController, mgmtoapi.ChiServerOptions{
		BaseRouter: router,
		// The generated wrapper applies these middlewares by wrapping forward
		// (handler = mw(handler)), so the LAST entry becomes the outermost and
		// runs FIRST. RateLimit is therefore listed before the validator so it
		// runs *after* authentication and keys on the resolved auth method; the
		// budget is shared with the client API, so a key cannot get a separate
		// allowance per surface.
		Middlewares: []mgmtoapi.MiddlewareFunc{
			http.RateLimit(limiter, cfg.RateLimit.PerMinute, time.Minute, cfg.RateLimit.TrustedProxyHops, mgmtoapi.WriteProblem),
			mgmtoapi.Validator(mgmtSpec, openapi3filter.Options{
				AuthenticationFunc: auth.Middleware(
					auth.WithAdminSession(mgmtStores, consoleSigner, cfg.PublicBaseURL(), logger),
					auth.WithKey(mgmtStores, auth.SurfaceManagement),
				),
			}),
			// Listed last so it is the OUTERMOST middleware and runs FIRST: a
			// legacy cookie has to become a console session before the validator
			// authenticates the request.
			mgmtoapi.MiddlewareFunc(auth.UpgradeLegacySession(
				mgmtController.AuthController.Verifier(),
				mgmtController.AuthController.Exchanger(),
				consoleSigner,
				logger,
			)),
		},
	})

	// Mount client routes with API Key only auth
	router.Group(func(r chi.Router) {
		r.Use(clientoapi.CORS())
		r.Options("/api/client/*", func(w nethttp.ResponseWriter, r *nethttp.Request) {})
		clientoapi.HandlerWithOptions(clientController, clientoapi.ChiServerOptions{
			BaseRouter: r,
			// Listed before the validator so it runs *after* authentication —
			// see the management mount above for why the order is reversed.
			Middlewares: []clientoapi.MiddlewareFunc{
				http.RateLimit(limiter, cfg.RateLimit.PerMinute, time.Minute, cfg.RateLimit.TrustedProxyHops, clientoapi.WriteProblem),
				clientoapi.Validator(clientSpec, openapi3filter.Options{
					AuthenticationFunc: auth.Middleware(
						auth.WithKey(mgmtStores, auth.SurfaceClient),
						auth.WithSession(mgmtStores, sessionSigner),
						auth.WithTrustedIssuer(mgmtStores, jwksCache),
					),
				}),
			},
		})
	})

	if cfg.Storage.BaseURL == "" {
		// Mount public (unauthenticated) document serving endpoint.
		// When using local storage without a CDN, this endpoint allows documents
		// to be loaded publicly (e.g. images embedded in emails).
		router.Get("/uploads/documents/{key}", documentsHandler(logger, storageDriver))
	}

	// Mount link tracking redirect endpoint (unauthenticated, outside OpenAPI validation).
	linkKey := cfg.Link.SecretBytes()
	linkPub := pubsub.NewPublisher(jet, cfg.Nats.Namespace)
	router.Get("/c/{token}", LinkRedirectHandler(logger, linkKey, linkPub))

	// Provider webhook endpoint (unauthenticated — providers can't send JWTs).
	router.Post("/webhooks/{projectID}/providers/{providerID}",
		ProviderWebhookHandler(logger, mgmtStores, registry, linkPub, cfg.Webhook.MaxBodySize, cfg.PublicBaseURL()),
	)

	// Serve static assets - use sub-filesystem to strip the "client/static" prefix
	staticSubFS, err := fs.Sub(staticFiles, "client/static")
	if err != nil {
		return nil, fmt.Errorf("failed to create static sub-filesystem: %w", err)
	}
	router.Handle("/static/*", nethttp.StripPrefix("/static/", nethttp.FileServer(nethttp.FS(staticSubFS))))

	// Mount enterprise proxy routes (backoffice, courier).
	// In OSS builds this is a no-op; in enterprise builds it registers
	// reverse proxy handlers based on PROXY_*_URL environment variables.
	MountProxyRoutes(logger, router, cfg.Enterprise)

	// Serve console (admin UI) as fallback
	consoleHandler, err := console.Handler(console.Config{
		ClerkPublishableKey: cfg.Console.ClerkPublishableKey,
	})
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

// documentsHandler returns an HTTP handler that serves document files from
// storage. This endpoint is unauthenticated so that documents can be loaded
// publicly (e.g. images embedded in emails).
func documentsHandler(logger *zap.Logger, s storage.Storage) nethttp.HandlerFunc {
	return func(w nethttp.ResponseWriter, r *nethttp.Request) {
		key := chi.URLParam(r, "key")
		if key == "" {
			nethttp.Error(w, "Not Found", nethttp.StatusNotFound)
			return
		}

		file, err := s.Read(r.Context(), key)
		if err != nil {
			logger.Debug("document not found in storage", zap.String("key", key), zap.Error(err))
			nethttp.Error(w, "Not Found", nethttp.StatusNotFound)
			return
		}
		defer file.Close()

		// Determine content type from the file extension in the key.
		contentType := mime.TypeByExtension(filepath.Ext(key))
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

		if _, err := io.Copy(w, file); err != nil {
			logger.Error("failed to write document to response", zap.String("key", key), zap.Error(err))
		}
	}
}
