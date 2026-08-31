package config

import (
	"time"

	"github.com/lunogram/platform/internal/http"
	"github.com/lunogram/platform/internal/mailer"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/storage"
	"github.com/lunogram/platform/internal/store"
)

// Defaults returns the configuration a deployment gets before its YAML file and
// its environment are applied.
//
// These were envDefault struct tags until configuration became layered. A tag
// default is applied by env.Parse whenever the variable is unset, which means it
// overwrites the YAML layer rather than yielding to it — the exact opposite of
// the documented precedence. Holding them in one constructor also makes the
// defaults readable as a set, which a tag per field never was.
func Defaults() Node {
	return Node{
		HTTPAddress:     ":8080",
		DatabaseMigrate: true,
		PublicURL:       "http://localhost:8080",

		Redis: Redis{
			Address: "redis://127.0.0.1:6379",
		},
		RateLimit: RateLimit{
			PerMinute:        600,
			TrustedProxyHops: 0,
		},
		JWKSCache: JWKSCache{
			TTL:          5 * time.Minute,
			FetchTimeout: 5 * time.Second,
			ErrorTTL:     30 * time.Second,
		},
		Cluster: Cluster{
			ReconciliationInterval:  time.Minute,
			ReconciliationBatchSize: 1000,
			LeaderCampaignInterval:  5 * time.Second,
			HeartbeatInterval:       4 * time.Second,
		},
		Auth: Auth{
			Password: PasswordAuth{
				Registration: RegistrationInviteOnly,
			},
			Console: ConsoleAuth{
				Issuer:      "https://lunogram.com/console",
				Audience:    "lunogram-console",
				IdleTTL:     8 * time.Hour,
				AbsoluteTTL: 168 * time.Hour,
			},
			LegacyIdentityAdoption: true,
			SessionIssuer:          "https://lunogram.com",
		},
		Nats: Nats{
			URL: "nats://127.0.0.1:4222",
		},
		WASM: WASM{
			CallTimeout: 30 * time.Second,
		},
		Webhook: Webhook{
			ProjectCreatedTimeout: 30 * time.Second,
			EmailTemplatesTimeout: 10 * time.Second,
			MaxBodySize:           1048576,
		},
		Mail:    mailer.DefaultConfig(),
		RBAC:    rbac.DefaultConfig(),
		HTTP:    http.DefaultConfig(),
		Store:   store.DefaultConfig(),
		Storage: storage.DefaultConfig(),
	}
}
