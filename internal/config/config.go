package config

import (
	"slices"
	"strings"
	"time"

	"github.com/lunogram/platform/internal/claim"
	"github.com/lunogram/platform/internal/http"
	"github.com/lunogram/platform/internal/mailer"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/storage"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/webhook"
)

type Node struct {
	NodeID          string `env:"NODE_ID" yaml:"node_id"`
	HTTPAddress     string `env:"HTTP_ADDRESS" yaml:"http_address"`
	DatabaseMigrate bool   `env:"DATABASE_MIGRATE" yaml:"database_migrate"`

	PublicURL  string         `env:"PUBLIC_URL" yaml:"public_url"`
	Redis      Redis          `envPrefix:"REDIS_" yaml:"redis"`
	RateLimit  RateLimit      `envPrefix:"RATE_LIMIT_" yaml:"rate_limit"`
	JWKSCache  JWKSCache      `envPrefix:"JWKS_CACHE_" yaml:"jwks_cache"`
	Cluster    Cluster        `envPrefix:"CLUSTER_" yaml:"cluster"`
	Auth       Auth           `envPrefix:"AUTH_" yaml:"auth"`
	Nats       Nats           `envPrefix:"NATS_" yaml:"nats"`
	WASM       WASM           `envPrefix:"WASM_" yaml:"wasm"`
	Webhook    Webhook        `envPrefix:"WEBHOOK_" yaml:"webhook"`
	Link       Link           `envPrefix:"LINK_" yaml:"link"`
	Mail       mailer.Config  `envPrefix:"MAIL_" yaml:"mail"`
	RBAC       rbac.Config    `envPrefix:"RBAC_" yaml:"rbac"`
	Enterprise Enterprise     `yaml:"enterprise"`
	Console    Console        `envPrefix:"CONSOLE_" yaml:"console"`
	HTTP       http.Config    `yaml:"http"`
	Store      store.Config   `yaml:"store"`
	Storage    storage.Config `yaml:"storage"`

	// baseDir is the directory the configuration file was read from, used to
	// resolve relative file:// references.
	baseDir string `yaml:"-"`
}

// PublicBaseURL returns the public URL with any trailing slash removed,
// suitable for concatenating with request paths.
func (n Node) PublicBaseURL() string {
	return strings.TrimRight(n.PublicURL, "/")
}

type Auth struct {
	// Drivers are the login methods this deployment offers. AUTH_DRIVER takes a
	// comma-separated list, so the documented single-driver quickstart
	// (AUTH_DRIVER=basic) is unchanged while a deployment that wants, say,
	// local accounts alongside SSO can say AUTH_DRIVER=basic,clerk. Every
	// configured driver is offered by GET /api/auth/methods and reachable at its
	// own callback; the console picks between them.
	Drivers []string    `env:"DRIVER" envSeparator:"," yaml:"drivers"`
	JWKS    claim.JWKS  `env:"JWKS_URL" yaml:"jwks"`
	Basic   BasicAuth   `envPrefix:"BASIC_" yaml:"basic"`
	Clerk   ClerkAuth   `envPrefix:"CLERK_" yaml:"clerk"`
	Console ConsoleAuth `envPrefix:"CONSOLE_" yaml:"console"`

	// LegacyIdentityAdoption lets an upstream identity claim a
	// pre-existing admin whose identity row still carries the sentinel issuer
	// the dropped admins.external_id column was backfilled under. It matches on
	// subject alone, so it is a transitional allowance rather than a permanent
	// resolution step; it ships disabled once no rows carry the sentinel.
	LegacyIdentityAdoption bool `env:"LEGACY_IDENTITY_ADOPTION" yaml:"legacy_identity_adoption"`

	// SessionSigningKey is a PEM-encoded EC (P-256) private key used to sign and
	// verify short-lived client session tokens (ES256). When empty, session
	// minting and verification are disabled.
	SessionSigningKey string `env:"SESSION_SIGNING_KEY" yaml:"session_signing_key"`
	// SessionIssuer is the `iss` stamped on (and required of) session tokens.
	SessionIssuer string `env:"SESSION_ISSUER" yaml:"session_issuer"`
}

// ConsoleAuth configures the Lunogram-issued console session: the credential
// every admin login is exchanged for. Its signing key is deliberately separate
// from the client SessionSigningKey. If one key signed both, the only thing
// separating a console token from a client one would be claim shape; with
// separate keys the boundary is cryptographic, and a console token fails
// signature verification at the client middleware before a single claim is read.
type ConsoleAuth struct {
	// SigningKey is a PEM-encoded EC (P-256) private key. Console sessions are
	// ES256.
	SigningKey string `env:"SIGNING_KEY" yaml:"signing_key"`
	// PreviousSigningKeys are retired keys that must still verify so a rotation
	// does not log everyone out. Comma-separated PEM blocks.
	PreviousSigningKeys []string `env:"PREVIOUS_SIGNING_KEYS" envSeparator:"," yaml:"previous_signing_keys"`
	// Issuer is distinct from the client session issuer so the two token
	// populations are never confusable, even before signatures are considered.
	Issuer string `env:"ISSUER" yaml:"issuer"`
	// Audience is required on every console token.
	Audience string `env:"AUDIENCE" yaml:"audience"`
	// IdleTTL is how long a session survives without being refreshed.
	IdleTTL time.Duration `env:"IDLE_TTL" yaml:"idle_ttl"`
	// AbsoluteTTL caps a session's total life however often it is refreshed.
	AbsoluteTTL time.Duration `env:"ABSOLUTE_TTL" yaml:"absolute_ttl"`
}

// Enabled reports whether the named login driver is configured.
//
// It compares against the normalised list, which is what [Normalise] exists for:
// the verifier registry trims and lower-cases every name as it builds, so an
// AUTH_DRIVER of " BASIC " used to build and advertise the driver while every
// check here said it was off.
func (a Auth) Enabled(driver string) bool {
	return slices.Contains(a.Drivers, driver)
}

// Normalise trims and lower-cases the configured driver names, dropping blanks
// and duplicates. It runs once at load so that every later comparison — here, in
// the verifier registry, and in the flows that ask whether their driver is on —
// is looking at the same spelling.
func (a *Auth) Normalise() {
	if len(a.Drivers) == 0 {
		return
	}

	var normalised []string
	for _, driver := range a.Drivers {
		driver = strings.ToLower(strings.TrimSpace(driver))
		if driver == "" || slices.Contains(normalised, driver) {
			continue
		}
		normalised = append(normalised, driver)
	}
	a.Drivers = normalised
}

// Configured reports whether any login driver is set up at all.
func (a Auth) Configured() bool { return len(a.Drivers) > 0 }

// BasicAuth configures local email and password credentials.
//
// Email and Password seed the first account. They are the documented quickstart
// and they are no longer a credential the login compares against: the pair is
// written into an admin at boot, hashed like any other, so the plaintext is
// needed once rather than for as long as the deployment runs. Remove it from the
// environment once the account exists.
type BasicAuth struct {
	Email    string `env:"EMAIL" yaml:"email"`
	Password string `env:"PASSWORD" yaml:"password"`

	// Registration decides who may create an account: open, invite_only or
	// disabled.
	Registration string `env:"REGISTRATION" yaml:"registration"`
}

// Registration modes for the password driver.
const (
	// RegistrationOpen lets anybody create an account. It is what a public SaaS
	// wants and what a private deployment must never be left on by accident.
	RegistrationOpen = "open"
	// RegistrationInviteOnly limits registration to addresses that already hold
	// a pending invite -- plus the very first account, which nobody could have
	// invited. It is the default because a deployment that has not thought
	// about the question should not be handing out accounts.
	RegistrationInviteOnly = "invite_only"
	// RegistrationDisabled turns the endpoint off entirely, for a deployment
	// that provisions its admins some other way.
	RegistrationDisabled = "disabled"
)

type ClerkAuth struct {
	SecretKey     string `env:"SECRET_KEY" yaml:"secret_key"`
	WebhookSecret string `env:"WEBHOOK_SECRET" yaml:"webhook_secret"`
	// Issuer is the `iss` this Clerk instance stamps on its session tokens
	// (e.g. https://your-app.clerk.accounts.dev). Logins take the issuer from
	// the verified token, so this is only needed by webhooks, whose payloads
	// carry no issuer: without it a user.created event is skipped and the admin
	// is provisioned by the exchange on first login instead.
	Issuer string `env:"ISSUER" yaml:"issuer"`
}

type Redis struct {
	Address   string `env:"ADDRESS" yaml:"address"`
	KeyPrefix string `env:"KEY_PREFIX" yaml:"key_prefix"`
}

// RateLimit configures request rate limiting across the API.
type RateLimit struct {
	// PerMinute is the number of API requests permitted per minute per auth
	// method (or per IP for unauthenticated requests). The budget is shared
	// across the client and management APIs — a key is not given a separate
	// allowance per surface.
	PerMinute int `env:"PER_MINUTE" yaml:"per_minute"`
	// TrustedProxyHops is the number of reverse proxies in front of the server.
	// X-Forwarded-For is honored only up to this many hops when deriving the
	// client IP for unauthenticated rate limiting; 0 (the default) ignores the
	// spoofable header and uses the connection's remote address.
	TrustedProxyHops int `env:"TRUSTED_PROXY_HOPS" yaml:"trusted_proxy_hops"`
}

// JWKSCache configures the two-tier cache for trusted-issuer JWKS verification.
type JWKSCache struct {
	// TTL is how long a fetched JWKS is kept in the shared (Redis) cache.
	TTL time.Duration `env:"TTL" yaml:"ttl"`
	// FetchTimeout bounds a single JWKS fetch from an issuer.
	FetchTimeout time.Duration `env:"FETCH_TIMEOUT" yaml:"fetch_timeout"`
	// ErrorTTL is the backoff after a failed fetch before the issuer is retried.
	ErrorTTL time.Duration `env:"ERROR_TTL" yaml:"error_ttl"`
}

type Nats struct {
	URL               string `env:"URL" yaml:"url"`
	Namespace         string `env:"NAMESPACE" yaml:"namespace"`
	ManagedExternally bool   `env:"MANAGED_EXTERNALLY" yaml:"managed_externally"`
}

type Cluster struct {
	ReconciliationInterval time.Duration `env:"RECONCILIATION_INTERVAL" yaml:"reconciliation_interval"`
	// ReconciliationBatchSize is the maximum number of rows each
	// reconciliation task (journey resumptions, scheduled messages,
	// inbox dispatch, broadcasts, list recomputation, ...) will scan
	// and process in a single tick. Lower values smooth out load at
	// the cost of higher end-to-end latency for large backlogs;
	// higher values drain backlogs faster but increase per-tick
	// resource usage. Remaining work rolls over to the next tick.
	ReconciliationBatchSize int           `env:"RECONCILIATION_BATCH_SIZE" yaml:"reconciliation_batch_size"`
	LeaderCampaignInterval  time.Duration `env:"LEADER_CAMPAIGN_INTERVAL" yaml:"leader_campaign_interval"`
	HeartbeatInterval       time.Duration `env:"HEARTBEAT_INTERVAL" yaml:"heartbeat_interval"`
}

type WASM struct {
	CallTimeout time.Duration `env:"CALL_TIMEOUT" yaml:"call_timeout"`
}

type Webhook struct {
	// ConfigFile is the path to the YAML file declaring outbound hooks. A hook
	// set cannot be expressed in environment variables — many subscribers per
	// event, each with its own template, credential, retry and network policy —
	// so it lives in a file, and this is the only environment variable involved
	// in reaching it. When empty, the deprecated single-URL variables below are
	// used instead.
	ConfigFile string `env:"CONFIG_FILE" yaml:"config_file"`

	// Outbound declares the hook engine inline, as the webhook.outbound section
	// of the node configuration file. It takes precedence over ConfigFile; a
	// deployment that already ships a standalone hooks file keeps working
	// unchanged, and a new one has a single file to write.
	Outbound *webhook.Config `yaml:"outbound"`

	// ProjectCreatedURL is the webhook URL called after project creation.
	//
	// Deprecated: use ConfigFile. Synthesised into an equivalent single-hook
	// configuration at boot; see [webhook.LegacyEnv].
	ProjectCreatedURL string `env:"PROJECT_CREATED_URL" yaml:"project_created_url"`
	// ProjectCreatedTimeout is the HTTP timeout for the webhook call.
	//
	// Deprecated: use ConfigFile.
	ProjectCreatedTimeout time.Duration `env:"PROJECT_CREATED_TIMEOUT" yaml:"project_created_timeout"`

	// LegacyForwardAuthorization restores the pre-engine behaviour of
	// forwarding the API caller's Authorization header to ProjectCreatedURL.
	//
	// Deprecated: the old implementation copied every inbound header, including
	// the caller's bearer token, onto a request to an operator-configured URL.
	// Forwarding is off by default now. This exists only as a bridge for a
	// receiver that has not yet been given its own credential.
	LegacyForwardAuthorization bool `env:"LEGACY_FORWARD_AUTHORIZATION" yaml:"legacy_forward_authorization"`

	// EmailTemplatesURL is the URL fetched for email starter templates.
	// When not configured, the endpoint returns an empty list.
	//
	// Deprecated: use ConfigFile's email_templates block.
	EmailTemplatesURL string `env:"EMAIL_TEMPLATES_URL" yaml:"email_templates_url"`
	// EmailTemplatesTimeout is the HTTP timeout for the gallery fetch.
	//
	// Deprecated: use ConfigFile's email_templates block.
	EmailTemplatesTimeout time.Duration `env:"EMAIL_TEMPLATES_TIMEOUT" yaml:"email_templates_timeout"`

	// MaxBodySize is the maximum allowed request body size in bytes for
	// inbound provider webhook payloads. Defaults to 1 MB. Unrelated to
	// outbound hooks.
	MaxBodySize int64 `env:"MAX_BODY_SIZE" yaml:"max_body_size"`
}

// Link holds the configuration for self-hosted click tracking.
// Both Secret (a 64-char hex string encoding a 32-byte AES-256 key) and
// TrackingURL (the public base URL for the redirect endpoint) must be set
// for link wrapping to be active.
type Link struct {
	// Secret is a 32-byte key encoded, used for
	// AES-256-GCM encryption of click-tracking tokens.
	Secret string `env:"SECRET" yaml:"secret"`
	// TrackingURL is the public base URL where the redirect endpoint is
	// reachable (e.g. "https://t.example.com"). When empty, the platform's
	// PublicURL is used instead.
	TrackingURL string `env:"TRACKING_URL" yaml:"tracking_url"`
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
	ClerkPublishableKey string `env:"CLERK_PUBLISHABLE_KEY" yaml:"clerk_publishable_key"`
}
