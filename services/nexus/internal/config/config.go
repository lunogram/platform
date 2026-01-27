package config

import (
	"time"

	"github.com/lunogram/platform/services/nexus/internal/claim"
	"github.com/lunogram/platform/services/nexus/internal/http"
	"github.com/lunogram/platform/services/nexus/internal/storage"
	"github.com/lunogram/platform/services/nexus/internal/store"
)

type Node struct {
	NodeID                   string  `env:"NODE_ID" envDefault:""`
	ManagementServiceAddress string  `env:"ADMIN_SERVICE_ADDRESS" envDefault:":8080"`
	PublicServiceAddress     string  `env:"PUBLIC_SERVICE_ADDRESS" envDefault:":8081"`
	PlatformURL              string  `env:"PLATFORM_URL" envDefault:"http://localhost:3001"`
	Redis                    Redis   `envPrefix:"REDIS_"`
	Cluster                  Cluster `envPrefix:"CLUSTER_"`
	Auth                     Auth    `envPrefix:"AUTH_"`
	Nats                     Nats    `envPrefix:"NATS_"`
	WASM                     WASM    `envPrefix:"WASM_"`
	HTTP                     http.Config
	Store                    store.Config
	Storage                  storage.Config
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
	URL string `env:"URL" envDefault:"nats://127.0.0.1:4222"`
}

type Cluster struct {
	ReconciliationInterval time.Duration `env:"RECONCILIATION_INTERVAL" envDefault:"1m"`
	LeaderCampaignInterval time.Duration `env:"LEADER_CAMPAIGN_INTERVAL" envDefault:"5s"`
	HeartbeatInterval      time.Duration `env:"HEARTBEAT_INTERVAL" envDefault:"4s"`
}

type WASM struct {
	CallTimeout time.Duration `env:"CALL_TIMEOUT" envDefault:"30s"`
}
