// Package sso holds the parts of the deployment's OpenID Connect login that are
// not HTTP: the provider metadata cache and the store of outstanding
// authorization requests.
//
// The login flow itself lives in internal/http/auth/verifiers, because a login
// is a credential being proved and that package is where verifiers live.
package sso

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/lunogram/platform/internal/ssrf"
	"golang.org/x/sync/singleflight"
)

var (
	// ErrIssuerMismatch reports that the discovery document does not identify
	// the issuer the deployment is configured for.
	ErrIssuerMismatch = errors.New("sso: the discovery document names a different issuer")
	// ErrDiscoveryOrigin reports that the discovery URL is not served by the
	// issuer's own origin.
	ErrDiscoveryOrigin = errors.New("sso: the discovery document must be served by the issuer's own origin")
	// ErrDiscoveryIncomplete reports that the document is missing an endpoint
	// the login flow cannot run without.
	ErrDiscoveryIncomplete = errors.New("sso: the discovery document is missing a required endpoint")
	// ErrDiscoveryInsecure reports that the document advertises an endpoint the
	// deployment's outbound policy refuses: plaintext, or an address that is not
	// a public one.
	ErrDiscoveryInsecure = errors.New("sso: the discovery document advertises an endpoint that is not reachable under the outbound policy")
)

// maxDiscoveryBytes caps a discovery document, so a misbehaving or hostile
// endpoint cannot be read into memory without bound.
const maxDiscoveryBytes = 1 << 20

// discoveryFetchTimeout bounds a coalesced fetch. The outbound client carries
// its own timeout, but the fetch context is deliberately detached from every
// caller, so it needs a deadline that does not depend on one being configured.
const discoveryFetchTimeout = 15 * time.Second

// discoveryTTL is how long a provider's metadata is kept in process. The
// endpoints in it change on the order of never; this is short enough that a
// provider migrating them is picked up without a restart.
const discoveryTTL = 15 * time.Minute

// Metadata is the part of an OpenID Connect discovery document the platform
// uses.
type Metadata struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
	// UserInfoEndpoint is called only when the ID token omits the configured
	// email claim, which OpenID Connect permits a provider to do. It is held to
	// the same outbound policy as the endpoints that are always used.
	UserInfoEndpoint  string   `json:"userinfo_endpoint"`
	SigningAlgorithms []string `json:"id_token_signing_alg_values_supported"`
	// TokenAuthMethods is how the provider expects the client to authenticate
	// at the token endpoint. OpenID Connect Discovery says an absent value means
	// client_secret_basic, so it is not safe to assume the other one.
	TokenAuthMethods []string `json:"token_endpoint_auth_methods_supported"`
}

// DefaultDiscoveryURL is where an issuer publishes its metadata unless it says
// otherwise.
func DefaultDiscoveryURL(issuer string) string {
	return strings.TrimSuffix(issuer, "/") + "/.well-known/openid-configuration"
}

type discoveryEntry struct {
	metadata Metadata
	expires  time.Time
}

// Discovery fetches and caches OpenID Connect provider metadata.
//
// It is a metadata cache and nothing more. The verification keys the metadata
// points at are cached by internal/jwks, which already solves rotation,
// coalescing and negative caching across the fleet; adding a second key cache
// beside it would mean two answers to "is this key still valid".
type Discovery struct {
	client       *http.Client
	ttl          time.Duration
	fetchTimeout time.Duration
	// policy is the same outbound guard the HTTP client dials under. Every
	// endpoint the document advertises is checked against it, because the
	// document -- not the operator -- is what names them.
	policy ssrf.Policy

	group   singleflight.Group
	mu      sync.RWMutex
	entries map[string]discoveryEntry
}

func NewDiscovery(client *http.Client, policy ssrf.Policy, ttl time.Duration) *Discovery {
	if ttl <= 0 {
		ttl = discoveryTTL
	}
	return &Discovery{
		client:       client,
		ttl:          ttl,
		fetchTimeout: discoveryFetchTimeout,
		policy:       policy,
		entries:      make(map[string]discoveryEntry),
	}
}

// Metadata returns the issuer's metadata, from cache when fresh.
//
// The document is bound to the issuer twice over, and both checks are load
// bearing. The document must be served by the issuer's OWN origin, and it must
// name the issuer as its own. Without them, whoever chooses the discovery URL
// chooses the token endpoint and the JWKS -- which is to say they can mint an ID
// token for any issuer they like, and the exact-match check go-oidc performs on
// the token's `iss` would pass against keys they control.
func (d *Discovery) Metadata(ctx context.Context, discoveryURL, issuer string) (Metadata, error) {
	if err := d.ValidateDiscoveryURL(discoveryURL, issuer); err != nil {
		return Metadata{}, err
	}

	// Keyed on the pair, not on the URL alone. Two issuers may legitimately
	// publish from the same origin, and a document cached for one of them has
	// only been checked against that one; serving it to the other -- or joining
	// its in-flight fetch -- would skip the issuer match below.
	key := discoveryURL + "\n" + issuer

	d.mu.RLock()
	entry, ok := d.entries[key]
	d.mu.RUnlock()
	if ok && time.Now().Before(entry.expires) {
		return entry.metadata, nil
	}

	// The fetch runs on a context no caller owns. Whichever login happens to
	// lead the singleflight may have its browser closed mid-flight, and with
	// the leader's context the cancellation would fail every other login that
	// coalesced behind it. Each caller still stops waiting on its own context.
	shared := d.group.DoChan(key, func() (any, error) {
		fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), d.fetchTimeout)
		defer cancel()

		metadata, err := d.fetch(fetchCtx, discoveryURL)
		if err != nil {
			return Metadata{}, err
		}
		if metadata.Issuer != issuer {
			return Metadata{}, fmt.Errorf("%w: %q", ErrIssuerMismatch, metadata.Issuer)
		}

		d.mu.Lock()
		d.entries[key] = discoveryEntry{metadata: metadata, expires: time.Now().Add(d.ttl)}
		d.mu.Unlock()
		return metadata, nil
	})

	select {
	case <-ctx.Done():
		return Metadata{}, ctx.Err()
	case result := <-shared:
		if result.Err != nil {
			return Metadata{}, result.Err
		}
		return result.Val.(Metadata), nil
	}
}

// fetch reads and validates a discovery document, without caching it and
// without checking it against a configured issuer. Only [Discovery.Metadata]
// calls it, and that is where the issuer binding lives.
func (d *Discovery) fetch(ctx context.Context, discoveryURL string) (Metadata, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return Metadata{}, err
	}

	response, err := d.client.Do(request)
	if err != nil {
		return Metadata{}, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return Metadata{}, fmt.Errorf("sso: discovery returned %s", response.Status)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxDiscoveryBytes+1))
	if err != nil {
		return Metadata{}, err
	}
	if len(body) > maxDiscoveryBytes {
		return Metadata{}, fmt.Errorf("sso: the discovery document exceeds %d bytes", maxDiscoveryBytes)
	}

	var metadata Metadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return Metadata{}, fmt.Errorf("sso: the discovery document is not valid JSON: %w", err)
	}

	if metadata.AuthorizationEndpoint == "" || metadata.TokenEndpoint == "" || metadata.JWKSURI == "" {
		return Metadata{}, ErrDiscoveryIncomplete
	}
	// The document decides where the authorization code and the client secret
	// are sent and which keys sign the ID token, so an endpoint in it is as
	// sensitive as one an operator configured and is held to the same policy.
	// Checking only that the fields are non-empty would accept a plaintext
	// token endpoint, or a JWKS inside the deployment's own network.
	for _, endpoint := range []string{
		metadata.AuthorizationEndpoint,
		metadata.TokenEndpoint,
		metadata.JWKSURI,
		metadata.UserInfoEndpoint,
	} {
		if endpoint == "" {
			continue
		}
		if err := ssrf.ValidateURL(endpoint, d.policy); err != nil {
			return Metadata{}, fmt.Errorf("%w: %q: %w", ErrDiscoveryInsecure, endpoint, err)
		}
	}
	return metadata, nil
}

// ValidateDiscoveryURL checks that a discovery URL is reachable under the
// deployment's outbound policy and is served by the issuer's own origin. It
// runs at startup and again on every login, so a document that moved off the
// issuer's origin stops being trusted rather than staying trusted because it
// passed once.
//
// The policy check is not redundant with the dialer's. SafeHTTPClient decides
// which hosts may be reached and says nothing about the scheme, so without this
// an `http://` issuer would have its metadata read off the wire unauthenticated
// -- and whoever can rewrite that response chooses the token endpoint and the
// JWKS, which is the whole attack the origin binding exists to prevent.
func (d *Discovery) ValidateDiscoveryURL(discoveryURL, issuer string) error {
	if err := ssrf.ValidateURL(issuer, d.policy); err != nil {
		return fmt.Errorf("%w: issuer %q: %w", ErrDiscoveryInsecure, issuer, err)
	}
	if err := ssrf.ValidateURL(discoveryURL, d.policy); err != nil {
		return fmt.Errorf("%w: %q: %w", ErrDiscoveryInsecure, discoveryURL, err)
	}
	return SameOrigin(discoveryURL, issuer)
}

// SameOrigin reports whether a discovery URL is served by the issuer's own
// origin.
func SameOrigin(discoveryURL, issuer string) error {
	discovery, err := url.Parse(discoveryURL)
	if err != nil {
		return fmt.Errorf("sso: invalid discovery url: %w", err)
	}
	issued, err := url.Parse(issuer)
	if err != nil {
		return fmt.Errorf("sso: invalid issuer url: %w", err)
	}

	if !strings.EqualFold(discovery.Scheme, issued.Scheme) ||
		!strings.EqualFold(discovery.Hostname(), issued.Hostname()) ||
		effectivePort(discovery) != effectivePort(issued) {
		return ErrDiscoveryOrigin
	}
	return nil
}

// effectivePort is the port a URL actually reaches, so that an issuer of
// https://idp.test and a discovery URL of https://idp.test:443/... are
// recognised as the one origin they are. Spelling the default port out is
// unusual but legal, and refusing it would be refusing a correct configuration.
func effectivePort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return "443"
	case "http":
		return "80"
	default:
		return ""
	}
}
