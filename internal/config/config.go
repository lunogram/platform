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
)

type Node struct {
	NodeID          string `env:"NODE_ID" envDefault:""`
	HTTPAddress     string `env:"HTTP_ADDRESS" envDefault:":8080"`
	DatabaseMigrate bool   `env:"DATABASE_MIGRATE" envDefault:"true"`

	PublicURL  string        `env:"PUBLIC_URL" envDefault:"http://localhost:8080"`
	Redis      Redis         `envPrefix:"REDIS_"`
	RateLimit  RateLimit     `envPrefix:"RATE_LIMIT_"`
	JWKSCache  JWKSCache     `envPrefix:"JWKS_CACHE_"`
	Cluster    Cluster       `envPrefix:"CLUSTER_"`
	Auth       Auth          `envPrefix:"AUTH_"`
	Nats       Nats          `envPrefix:"NATS_"`
	WASM       WASM          `envPrefix:"WASM_"`
	Webhook    Webhook       `envPrefix:"WEBHOOK_"`
	Link       Link          `envPrefix:"LINK_"`
	Mail       mailer.Config `envPrefix:"MAIL_"`
	RBAC       rbac.Config   `envPrefix:"RBAC_"`
	Enterprise Enterprise
	Console    Console `envPrefix:"CONSOLE_"`
	HTTP       http.Config
	Store      store.Config
	Storage    storage.Config
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
	// passwords alongside SSO can say AUTH_DRIVER=password,clerk. Every
	// configured driver is offered by GET /api/auth/methods and reachable at its
	// own callback; the console picks between them.
	Drivers  []string     `env:"DRIVER" envSeparator:","`
	JWKS     claim.JWKS   `env:"JWKS_URL"`
	Basic    BasicAuth    `envPrefix:"BASIC_"`
	Clerk    ClerkAuth    `envPrefix:"CLERK_"`
	Password PasswordAuth `envPrefix:"PASSWORD_"`
	Console  ConsoleAuth  `envPrefix:"CONSOLE_"`

	// LegacyIdentityAdoption lets an upstream identity claim a
	// pre-existing admin whose identity row still carries the sentinel issuer
	// the dropped admins.external_id column was backfilled under. It matches on
	// subject alone, so it is a transitional allowance rather than a permanent
	// resolution step; it ships disabled once no rows carry the sentinel.
	LegacyIdentityAdoption bool `env:"LEGACY_IDENTITY_ADOPTION" envDefault:"true"`

	// SessionSigningKey is a PEM-encoded EC (P-256) private key used to sign and
	// verify short-lived client session tokens (ES256). When empty, session
	// minting and verification are disabled.
	SessionSigningKey string `env:"SESSION_SIGNING_KEY"`
	// SessionIssuer is the `iss` stamped on (and required of) session tokens.
	SessionIssuer string `env:"SESSION_ISSUER" envDefault:"https://lunogram.com"`
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
	SigningKey string `env:"SIGNING_KEY"`
	// PreviousSigningKeys are retired keys that must still verify so a rotation
	// does not log everyone out. Comma-separated PEM blocks.
	PreviousSigningKeys []string `env:"PREVIOUS_SIGNING_KEYS" envSeparator:","`
	// Issuer is distinct from the client session issuer so the two token
	// populations are never confusable, even before signatures are considered.
	Issuer string `env:"ISSUER" envDefault:"https://lunogram.com/console"`
	// Audience is required on every console token.
	Audience string `env:"AUDIENCE" envDefault:"lunogram-console"`
	// IdleTTL is how long a session survives without being refreshed.
	IdleTTL time.Duration `env:"IDLE_TTL" envDefault:"8h"`
	// AbsoluteTTL caps a session's total life however often it is refreshed.
	AbsoluteTTL time.Duration `env:"ABSOLUTE_TTL" envDefault:"168h"`
}

// Enabled reports whether the named login driver is configured.
func (a Auth) Enabled(driver string) bool {
	return slices.Contains(a.Drivers, driver)
}

// Configured reports whether any login driver is set up at all.
func (a Auth) Configured() bool { return len(a.Drivers) > 0 }

type BasicAuth struct {
	Email    string `env:"EMAIL"`
	Password string `env:"PASSWORD"`
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

// PasswordAuth configures local email/password credentials.
type PasswordAuth struct {
	// Registration decides who may create an account: open, invite_only or
	// disabled.
	Registration string `env:"REGISTRATION" envDefault:"invite_only"`
}

type ClerkAuth struct {
	SecretKey     string `env:"SECRET_KEY"`
	WebhookSecret string `env:"WEBHOOK_SECRET"`
	// Issuer is the `iss` this Clerk instance stamps on its session tokens
	// (e.g. https://your-app.clerk.accounts.dev). Logins take the issuer from
	// the verified token, so this is only needed by webhooks, whose payloads
	// carry no issuer: without it a user.created event is skipped and the admin
	// is provisioned by the exchange on first login instead.
	Issuer string `env:"ISSUER"`
}

type Redis struct {
	Address   string `env:"ADDRESS" envDefault:"redis://127.0.0.1:6379"`
	KeyPrefix string `env:"KEY_PREFIX" envDefault:""`
}

// RateLimit configures request rate limiting across the API.
type RateLimit struct {
	// PerMinute is the number of API requests permitted per minute per auth
	// method (or per IP for unauthenticated requests). The budget is shared
	// across the client and management APIs — a key is not given a separate
	// allowance per surface.
	PerMinute int `env:"PER_MINUTE" envDefault:"600"`
	// TrustedProxyHops is the number of reverse proxies in front of the server.
	// X-Forwarded-For is honored only up to this many hops when deriving the
	// client IP for unauthenticated rate limiting; 0 (the default) ignores the
	// spoofable header and uses the connection's remote address.
	TrustedProxyHops int `env:"TRUSTED_PROXY_HOPS" envDefault:"0"`
}

// JWKSCache configures the two-tier cache for trusted-issuer JWKS verification.
type JWKSCache struct {
	// TTL is how long a fetched JWKS is kept in the shared (Redis) cache.
	TTL time.Duration `env:"TTL" envDefault:"5m"`
	// FetchTimeout bounds a single JWKS fetch from an issuer.
	FetchTimeout time.Duration `env:"FETCH_TIMEOUT" envDefault:"5s"`
	// ErrorTTL is the backoff after a failed fetch before the issuer is retried.
	ErrorTTL time.Duration `env:"ERROR_TTL" envDefault:"30s"`
}

type Nats struct {
	URL               string `env:"URL" envDefault:"nats://127.0.0.1:4222"`
	Namespace         string `env:"NAMESPACE" envDefault:""`
	ManagedExternally bool   `env:"MANAGED_EXTERNALLY" envDefault:"false"`
}

type Cluster struct {
	ReconciliationInterval time.Duration `env:"RECONCILIATION_INTERVAL" envDefault:"1m"`
	// ReconciliationBatchSize is the maximum number of rows each
	// reconciliation task (journey resumptions, scheduled messages,
	// inbox dispatch, broadcasts, list recomputation, ...) will scan
	// and process in a single tick. Lower values smooth out load at
	// the cost of higher end-to-end latency for large backlogs;
	// higher values drain backlogs faster but increase per-tick
	// resource usage. Remaining work rolls over to the next tick.
	ReconciliationBatchSize int           `env:"RECONCILIATION_BATCH_SIZE" envDefault:"1000"`
	LeaderCampaignInterval  time.Duration `env:"LEADER_CAMPAIGN_INTERVAL" envDefault:"5s"`
	HeartbeatInterval       time.Duration `env:"HEARTBEAT_INTERVAL" envDefault:"4s"`
}

type WASM struct {
	CallTimeout time.Duration `env:"CALL_TIMEOUT" envDefault:"30s"`
}

type Webhook struct {
	// ConfigFile is the path to the YAML file declaring outbound hooks. A hook
	// set cannot be expressed in environment variables — many subscribers per
	// event, each with its own template, credential, retry and network policy —
	// so it lives in a file, and this is the only environment variable involved
	// in reaching it. When empty, the deprecated single-URL variables below are
	// used instead.
	ConfigFile string `env:"CONFIG_FILE"`

	// ProjectCreatedURL is the webhook URL called after project creation.
	//
	// Deprecated: use ConfigFile. Synthesised into an equivalent single-hook
	// configuration at boot; see [webhook.LegacyEnv].
	ProjectCreatedURL string `env:"PROJECT_CREATED_URL"`
	// ProjectCreatedTimeout is the HTTP timeout for the webhook call.
	//
	// Deprecated: use ConfigFile.
	ProjectCreatedTimeout time.Duration `env:"PROJECT_CREATED_TIMEOUT" envDefault:"30s"`

	// LegacyForwardAuthorization restores the pre-engine behaviour of
	// forwarding the API caller's Authorization header to ProjectCreatedURL.
	//
	// Deprecated: the old implementation copied every inbound header, including
	// the caller's bearer token, onto a request to an operator-configured URL.
	// Forwarding is off by default now. This exists only as a bridge for a
	// receiver that has not yet been given its own credential.
	LegacyForwardAuthorization bool `env:"LEGACY_FORWARD_AUTHORIZATION" envDefault:"false"`

	// EmailTemplatesURL is the URL fetched for email starter templates.
	// When not configured, the endpoint returns an empty list.
	//
	// Deprecated: use ConfigFile's email_templates block.
	EmailTemplatesURL string `env:"EMAIL_TEMPLATES_URL"`
	// EmailTemplatesTimeout is the HTTP timeout for the gallery fetch.
	//
	// Deprecated: use ConfigFile's email_templates block.
	EmailTemplatesTimeout time.Duration `env:"EMAIL_TEMPLATES_TIMEOUT" envDefault:"10s"`

	// MaxBodySize is the maximum allowed request body size in bytes for
	// inbound provider webhook payloads. Defaults to 1 MB. Unrelated to
	// outbound hooks.
	MaxBodySize int64 `env:"MAX_BODY_SIZE" envDefault:"1048576"`
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
