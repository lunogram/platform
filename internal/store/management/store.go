package management

import (
	"time"

	iredis "github.com/lunogram/platform/internal/redis"
	"github.com/lunogram/platform/internal/store"
	goredis "github.com/redis/go-redis/v9"
)

// authCacheTTL bounds how long resolved auth credentials (API keys, trusted
// issuers) live in the shared cache. Writes invalidate the affected entries
// explicitly; this TTL is only the backstop for a missed invalidation.
const authCacheTTL = 5 * time.Minute

type stateConfig struct {
	apiKeyCache *iredis.Cache[APIKey]
	issuerCache *iredis.Cache[TrustedIssuerAuthMethod]
}

// StateOption configures a State at construction.
type StateOption func(*stateConfig)

// WithRedis enables Redis-backed read-through caching of the hot auth lookups
// (API key by secret hash, trusted issuer by iss), shared across processes via
// the given client and key prefix. Without it the store reads straight from
// Postgres. Both the auth (read) and management (write/invalidate) States must
// be built with the same client+prefix for invalidation to be observed.
func WithRedis(client *goredis.Client, prefix string) StateOption {
	return func(c *stateConfig) {
		c.apiKeyCache = iredis.NewCache[APIKey](client, prefix+"auth:apikey:", authCacheTTL, nil)
		c.issuerCache = iredis.NewCache[TrustedIssuerAuthMethod](client, prefix+"auth:issuer:", authCacheTTL, nil)
	}
}

func NewState(db store.DB, opts ...StateOption) *State {
	var cfg stateConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	return &State{
		AdminsStore:               NewAdminsStore(db),
		ProjectsStore:             NewProjectsStore(db),
		CampaignsStore:            NewCampaignsStore(db),
		ProvidersStore:            NewProvidersStore(db),
		TemplatesStore:            NewTemplatesStore(db),
		SubscriptionsStore:        NewSubscriptionsStore(db),
		OrganizationsStore:        NewOrganizationsStore(db),
		TagsStore:                 NewTagsStore(db),
		LocalesStore:              NewLocalesStore(db),
		DocumentsStore:            NewDocumentsStore(db),
		AuthStore:                 NewAuthStore(db, cfg.apiKeyCache, cfg.issuerCache),
		AuthMethodsStore:          NewAuthMethodsStore(db, cfg.apiKeyCache, cfg.issuerCache),
		ActionsStore:              NewActionsStore(db),
		SenderIdentitiesStore:     NewSenderIdentitiesStore(db),
		BroadcastsStore:           NewBroadcastsStore(db),
		ProjectPushProvidersStore: NewProjectPushProvidersStore(db),
		VapidKeysStore:            NewVapidKeysStore(db),
	}
}

type State struct {
	*AdminsStore
	*ProjectsStore
	*CampaignsStore
	*ProvidersStore
	*TemplatesStore
	*SubscriptionsStore
	*OrganizationsStore
	*TagsStore
	*LocalesStore
	*DocumentsStore
	*AuthStore
	*AuthMethodsStore
	*ActionsStore
	*SenderIdentitiesStore
	*BroadcastsStore
	*ProjectPushProvidersStore
	*VapidKeysStore
}
