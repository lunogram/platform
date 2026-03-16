package config

import (
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

	PublicURL string      `env:"PUBLIC_URL" envDefault:"http://localhost:8080"`
	Redis     Redis       `envPrefix:"REDIS_"`
	Cluster   Cluster     `envPrefix:"CLUSTER_"`
	Auth      Auth        `envPrefix:"AUTH_"`
	Nats      Nats        `envPrefix:"NATS_"`
	WASM      WASM        `envPrefix:"WASM_"`
	Webhook   Webhook     `envPrefix:"WEBHOOK_"`
	RBAC      rbac.Config `envPrefix:"RBAC_"`
	HTTP      http.Config
	Store     store.Config
	Storage   storage.Config
	Modules   []string `env:"MODULES" envSeparator:"," envDefault:"http,consumers,wasm,scheduler"`
}

var ValidModules = []string{"http", "consumers", "wasm", "scheduler"}

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
	URL       string `env:"URL" envDefault:"nats://127.0.0.1:4222"`
	Namespace string `env:"NAMESPACE" envDefault:""`
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
}
