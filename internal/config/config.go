package config

import (
	"strings"
	"time"

	"github.com/lunogram/platform/internal/claim"
	"github.com/lunogram/platform/internal/http"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/storage"
	"github.com/lunogram/platform/internal/store"
)

type Node struct {
	NodeID          string `env:"NODE_ID" envDefault:""`
	HTTPAddress     string `env:"HTTP_ADDRESS" envDefault:":8080"`
	DatabaseMigrate bool   `env:"DATABASE_MIGRATE" envDefault:"true"`

	PublicURL  string      `env:"PUBLIC_URL" envDefault:"http://localhost:8080"`
	Redis      Redis       `envPrefix:"REDIS_"`
	Cluster    Cluster     `envPrefix:"CLUSTER_"`
	Auth       Auth        `envPrefix:"AUTH_"`
	Nats       Nats        `envPrefix:"NATS_"`
	WASM       WASM        `envPrefix:"WASM_"`
	Webhook    Webhook     `envPrefix:"WEBHOOK_"`
	Link       Link        `envPrefix:"LINK_"`
	RBAC       rbac.Config `envPrefix:"RBAC_"`
	Enterprise Enterprise
	Console    Console `envPrefix:"CONSOLE_"`
	HTTP       http.Config
	Store      store.Config
	Storage    storage.Config
	Invites    Invites `envPrefix:"INVITES_"`
}

// PublicBaseURL returns the public URL with any trailing slash removed,
// suitable for concatenating with request paths.
func (n Node) PublicBaseURL() string {
	return strings.TrimRight(n.PublicURL, "/")
}

type Auth struct {
	Driver    string        `env:"DRIVER"`
	JWTSecret string        `env:"JWT_SECRET"`
	JWKS      claim.JWKS    `env:"JWKS_URL"`
	TokenLife time.Duration `env:"TOKEN_LIFE" envDefault:"24h"`
	Basic     BasicAuth     `envPrefix:"BASIC_"`
	Clerk     ClerkAuth     `envPrefix:"CLERK_"`
}

type BasicAuth struct {
	Email    string `env:"EMAIL"`
	Password string `env:"PASSWORD"`
}

type ClerkAuth struct {
	SecretKey     string `env:"SECRET_KEY"`
	WebhookSecret string `env:"WEBHOOK_SECRET"`
}

type Redis struct {
	Address   string `env:"ADDRESS" envDefault:"redis://127.0.0.1:6379"`
	KeyPrefix string `env:"KEY_PREFIX" envDefault:""`
}

type Nats struct {
	URL               string `env:"URL" envDefault:"nats://127.0.0.1:4222"`
	Namespace         string `env:"NAMESPACE" envDefault:""`
	ManagedExternally bool   `env:"MANAGED_EXTERNALLY" envDefault:"false"`
}

type Cluster struct {
	ReconciliationInterval time.Duration `env:"RECONCILIATION_INTERVAL" envDefault:"1m"`
	LeaderCampaignInterval time.Duration `env:"LEADER_CAMPAIGN_INTERVAL" envDefault:"5s"`
	HeartbeatInterval      time.Duration `env:"HEARTBEAT_INTERVAL" envDefault:"4s"`
}

type WASM struct {
	CallTimeout time.Duration `env:"CALL_TIMEOUT" envDefault:"30s"`
}

type Webhook struct {
	// ProjectCreatedURL is the webhook URL called after project creation
	ProjectCreatedURL string `env:"PROJECT_CREATED_URL"`
	// ProjectCreatedTimeout is the HTTP timeout for the webhook call
	ProjectCreatedTimeout time.Duration `env:"PROJECT_CREATED_TIMEOUT" envDefault:"30s"`

	// EmailTemplatesURL is the webhook URL called to fetch email starter templates.
	// When configured, the backend proxies gallery requests to this endpoint.
	// When not configured, the endpoint returns an empty list.
	EmailTemplatesURL string `env:"EMAIL_TEMPLATES_URL"`
	// EmailTemplatesTimeout is the HTTP timeout for the email templates webhook call
	EmailTemplatesTimeout time.Duration `env:"EMAIL_TEMPLATES_TIMEOUT" envDefault:"10s"`

	// MaxBodySize is the maximum allowed request body size in bytes for
	// inbound provider webhook payloads. Defaults to 1 MB.
	MaxBodySize int64 `env:"MAX_BODY_SIZE" envDefault:"1048576"`
}

type Invites struct {
	// SecretKey is the secret used to sign invite tokens. It should be a
	// secure random string in production, but can be a hardcoded value for
	// development and testing.
	SecretKey string `env:"SECRET_KEY" envDefault:"none"`
}

// Link holds the configuration for self-hosted click tracking.
// Both Secret (a 64-char hex string encoding a 32-byte AES-256 key) and
// TrackingURL (the public base URL for the redirect endpoint) must be set
// for link wrapping to be active.
type Link struct {
	// Secret is a 32-byte key encoded, used for
	// AES-256-GCM encryption of click-tracking tokens.
	Secret string `env:"SECRET"`
	// TrackingURL is the public base URL where the redirect endpoint is
	// reachable (e.g. "https://t.example.com"). When empty, the platform's
	// PublicURL is used instead.
	TrackingURL string `env:"TRACKING_URL"`
}

// SecretBytes decodes the hex-encoded secret into raw bytes.
func (l Link) SecretBytes() []byte {
	if l.Secret == "" {
		return nil
	}

	key := make([]byte, 32)
	copy(key, []byte(l.Secret))
	return key
}

// TrackingBaseURL returns the base URL for click-tracking redirects,
// with any trailing slash removed.
func (l Link) TrackingBaseURL() string {
	return strings.TrimRight(l.TrackingURL, "/")
}

type Console struct {
	// ClerkPublishableKey is the Clerk publishable key injected into the
	// console frontend at runtime via /config.js. This allows different
	// environments (stg, prd) to use different Clerk instances without
	// rebuilding the Docker image.
	ClerkPublishableKey string `env:"CLERK_PUBLISHABLE_KEY"`
}
