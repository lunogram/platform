// Package jwks provides a two-tier cache for JSON Web Key Sets used to verify
// externally-issued JWTs (the trusted-issuer access policy).
//
// The hot path never touches the network: a per-process L1 cache holds parsed
// verification keys (short TTL), backed by a shared L2 (the centralised
// Redis-backed [redis.Cache]) holding the raw JWKS JSON (long TTL) so a fleet
// hits each issuer at most ~once per L2 TTL. The parsed keys cannot live in
// Redis, so the L1 is unavoidable; it is kept short because key rotation is
// picked up out-of-band (an unknown key id triggers an immediate refresh)
// rather than by L1 expiry. Fetches are coalesced with singleflight and a fetch
// failure fails open to the last-known-good keys with a short negative-cache
// backoff, so a flapping issuer can neither block authentication nor be hammered.
package jwks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	iredis "github.com/lunogram/platform/internal/redis"
	"github.com/lunogram/platform/internal/ssrf"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

// maxJWKSBytes caps the size of a fetched JWKS document to avoid unbounded reads
// from a misbehaving issuer endpoint.
const maxJWKSBytes = 1 << 20 // 1 MiB

// readAll reads up to limit+1 bytes, erroring if the limit is exceeded.
func readAll(r io.Reader, limit int64) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("jwks: document exceeds %d bytes", limit)
	}
	return b, nil
}

// Config tunes the cache. Zero values fall back to sane defaults via withDefaults.
// The L2 (Redis) TTL is configured on the [redis.Cache] passed to New, not here.
type Config struct {
	// L1TTL is how long parsed keys are kept in-process. It should be short so
	// L2/issuer updates propagate quickly; it defaults to 5m.
	L1TTL time.Duration
	// FetchTimeout bounds a single issuer fetch.
	FetchTimeout time.Duration
	// ErrorTTL is the backoff after a failed fetch before the issuer is hit
	// again (the negative cache window).
	ErrorTTL time.Duration
	// MaxStale bounds how long last-known-good keys may be served while the
	// issuer is unreachable. Past this window the cache fails closed rather than
	// trusting indefinitely-stale keys, so an issuer that rotates away a
	// compromised key (or an attacker who disrupts the JWKS endpoint) cannot pin
	// the old key set forever. Defaults to 1h.
	MaxStale time.Duration
}

func (c Config) withDefaults() Config {
	if c.L1TTL <= 0 {
		c.L1TTL = 5 * time.Minute
	}
	if c.FetchTimeout <= 0 {
		c.FetchTimeout = 5 * time.Second
	}
	if c.ErrorTTL <= 0 {
		c.ErrorTTL = 30 * time.Second
	}
	if c.MaxStale <= 0 {
		c.MaxStale = time.Hour
	}
	return c
}

// Fetcher retrieves the raw JWKS document for a URL.
type Fetcher interface {
	Fetch(ctx context.Context, url string) ([]byte, error)
}

type entry struct {
	kf          keyfunc.Keyfunc // parsed verification keys; nil until first success
	expires     time.Time       // L1 freshness deadline
	lastError   time.Time       // most recent fetch failure (negative-cache marker)
	refreshedAt time.Time       // last successful fetch (staleness deadline anchor)
}

// Cache resolves a JWKS URL to a jwt.Keyfunc, fronting the network with an L1
// (parsed keys) and a shared L2 (raw JWKS JSON in Redis). It is safe for
// concurrent use.
type Cache struct {
	cfg     Config
	fetcher Fetcher
	l2      *iredis.Cache[json.RawMessage]
	logger  *zap.Logger

	group singleflight.Group
	mu    sync.RWMutex
	l1    map[string]*entry
}

// New constructs a cache. l2 is the shared Redis-backed L2 (a nil-client cache
// is fine and yields L1-only operation). When fetcher is nil a default
// SSRF-hardened HTTP fetcher bounded by cfg.FetchTimeout is used.
func New(cfg Config, l2 *iredis.Cache[json.RawMessage], fetcher Fetcher, logger *zap.Logger) *Cache {
	cfg = cfg.withDefaults()
	if fetcher == nil {
		fetcher = &httpFetcher{timeout: cfg.FetchTimeout, client: ssrf.SafeHTTPClient(cfg.FetchTimeout)}
	}
	return &Cache{
		cfg:     cfg,
		fetcher: fetcher,
		l2:      l2,
		logger:  logger,
		l1:      make(map[string]*entry),
	}
}

// Keyfunc returns a jwt.Keyfunc for the issuer's JWKS at url, using cached keys
// when fresh and fetching otherwise.
func (c *Cache) Keyfunc(ctx context.Context, url string) (jwt.Keyfunc, error) {
	c.mu.RLock()
	e := c.l1[url]
	var fresh keyfunc.Keyfunc
	if e != nil && e.kf != nil && time.Now().Before(e.expires) {
		fresh = e.kf
	}
	c.mu.RUnlock()

	if fresh != nil {
		return fresh.Keyfunc, nil
	}
	return c.load(ctx, url, false)
}

// Refresh forces a re-fetch of the issuer's JWKS, bypassing the L1/L2 freshness
// windows. Call this when a token presents a key id absent from the cached set,
// so rotated-in keys are picked up immediately.
func (c *Cache) Refresh(ctx context.Context, url string) (jwt.Keyfunc, error) {
	return c.load(ctx, url, true)
}

// load resolves keys for url, coalescing concurrent callers. When force is true
// the L1/L2 caches are bypassed for reads (but still written).
func (c *Cache) load(ctx context.Context, url string, force bool) (jwt.Keyfunc, error) {
	v, err, _ := c.group.Do(url+boolKey(force), func() (any, error) {
		if !force {
			// Another goroutine may have populated L1 while we queued.
			c.mu.RLock()
			e := c.l1[url]
			c.mu.RUnlock()
			if e != nil && e.kf != nil && time.Now().Before(e.expires) {
				return e.kf, nil
			}
			// Back off if we recently failed and still have nothing usable.
			if e != nil && e.kf == nil && time.Since(e.lastError) < c.cfg.ErrorTTL {
				return nil, fmt.Errorf("jwks: issuer %s recently failed, backing off", url)
			}

			if raw, ok := c.l2.Get(ctx, url); ok {
				if kf, err := keyfunc.NewJWKSetJSON(raw); err == nil {
					c.storeL1(url, kf)
					return kf, nil
				}
			}
		}

		raw, err := c.fetcher.Fetch(ctx, url)
		if err != nil {
			return c.handleFetchError(url, err)
		}

		kf, err := keyfunc.NewJWKSetJSON(json.RawMessage(raw))
		if err != nil {
			return c.handleFetchError(url, fmt.Errorf("parse jwks: %w", err))
		}

		c.l2.Set(ctx, url, json.RawMessage(raw))
		c.storeL1(url, kf)
		return kf, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(keyfunc.Keyfunc).Keyfunc, nil
}

// handleFetchError fails open to the last-known-good keys when available, and
// records the failure for negative caching otherwise. On fail-open it returns
// the stale keys with a nil error; with no cached keys it returns the error.
func (c *Cache) handleFetchError(url string, err error) (keyfunc.Keyfunc, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	e := c.l1[url]
	if e == nil {
		e = &entry{}
		c.l1[url] = e
	}
	now := time.Now()
	e.lastError = now

	if e.kf != nil {
		// Stop trusting keys that have gone stale past the bound: fail closed
		// rather than serving an indefinitely-old key set the issuer may have
		// rotated away.
		if now.Sub(e.refreshedAt) > c.cfg.MaxStale {
			if c.logger != nil {
				c.logger.Error("jwks fetch failed and keys exceeded max staleness, failing closed",
					zap.String("url", url), zap.Duration("stale_for", now.Sub(e.refreshedAt)), zap.Error(err))
			}
			return nil, fmt.Errorf("jwks: keys for %s exceeded max staleness (%s): %w", url, c.cfg.MaxStale, err)
		}
		// Serve stale keys; extend their L1 freshness briefly so we don't refetch
		// on every request while the issuer is down.
		e.expires = now.Add(c.cfg.ErrorTTL)
		if c.logger != nil {
			c.logger.Warn("jwks fetch failed, serving last-known-good keys", zap.String("url", url), zap.Error(err))
		}
		return e.kf, nil
	}

	if c.logger != nil {
		c.logger.Warn("jwks fetch failed, no cached keys", zap.String("url", url), zap.Error(err))
	}
	return nil, fmt.Errorf("jwks: fetch %s: %w", url, err)
}

func (c *Cache) storeL1(url string, kf keyfunc.Keyfunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	c.l1[url] = &entry{kf: kf, expires: now.Add(c.cfg.L1TTL), refreshedAt: now}
}

func boolKey(b bool) string {
	if b {
		return "\x00refresh"
	}
	return ""
}

// httpFetcher fetches JWKS over HTTP with a per-request timeout, using an
// SSRF-hardened client (see ssrf.SafeHTTPClient).
type httpFetcher struct {
	timeout time.Duration
	client  *http.Client
}

func (f *httpFetcher) Fetch(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return readAll(resp.Body, maxJWKSBytes)
}
