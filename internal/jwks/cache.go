// Package jwks provides a two-tier cache for JSON Web Key Sets used to verify
// externally-issued JWTs (the trusted-issuer access policy).
//
// The hot path never touches the network: a per-process L1 cache holds parsed
// verification keys, backed by an optional shared L2 (Redis) holding the raw
// JWKS JSON so a fleet hits each issuer at most ~once per TTL. Fetches are
// coalesced with singleflight, an unknown key id triggers an immediate refresh
// (so key rotation is picked up regardless of TTL), and fetch failures fail open
// to the last-known-good keys with a short negative-cache backoff so a flapping
// issuer can neither block authentication nor be hammered.
package jwks

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
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
type Config struct {
	// TTL is how long raw JWKS are kept in the shared L2 cache.
	TTL time.Duration
	// L1TTL is how long parsed keys are kept in-process. It should be short so
	// L2/issuer updates propagate quickly; it defaults to min(TTL, 30s).
	L1TTL time.Duration
	// FetchTimeout bounds a single issuer fetch.
	FetchTimeout time.Duration
	// ErrorTTL is the backoff after a failed fetch before the issuer is hit
	// again (the negative cache window).
	ErrorTTL time.Duration
}

func (c Config) withDefaults() Config {
	if c.TTL <= 0 {
		c.TTL = 5 * time.Minute
	}
	if c.L1TTL <= 0 {
		c.L1TTL = 30 * time.Second
		if c.TTL < c.L1TTL {
			c.L1TTL = c.TTL
		}
	}
	if c.FetchTimeout <= 0 {
		c.FetchTimeout = 5 * time.Second
	}
	if c.ErrorTTL <= 0 {
		c.ErrorTTL = 30 * time.Second
	}
	return c
}

// Fetcher retrieves the raw JWKS document for a URL.
type Fetcher interface {
	Fetch(ctx context.Context, url string) ([]byte, error)
}

// Store is the shared L2 cache (e.g. Redis). A nil Store disables L2 and the
// cache operates L1-only.
type Store interface {
	Get(ctx context.Context, key string) (value []byte, ok bool, err error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

type entry struct {
	kf        keyfunc.Keyfunc // parsed verification keys; nil until first success
	expires   time.Time       // L1 freshness deadline
	lastError time.Time       // most recent fetch failure (negative-cache marker)
}

// Cache resolves a JWKS URL to a jwt.Keyfunc, fronting the network with L1/L2
// caches. It is safe for concurrent use.
type Cache struct {
	cfg     Config
	fetcher Fetcher
	store   Store
	logger  *zap.Logger

	group singleflight.Group
	mu    sync.RWMutex
	l1    map[string]*entry
}

// New constructs a cache. store may be nil (L1-only). When fetcher is nil a
// default HTTP fetcher bounded by cfg.FetchTimeout is used.
func New(cfg Config, store Store, fetcher Fetcher, logger *zap.Logger) *Cache {
	cfg = cfg.withDefaults()
	if fetcher == nil {
		fetcher = &httpFetcher{timeout: cfg.FetchTimeout}
	}
	return &Cache{
		cfg:     cfg,
		fetcher: fetcher,
		store:   store,
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

			if raw, ok := c.l2Get(ctx, url); ok {
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

		kf, err := keyfunc.NewJWKSetJSON(raw)
		if err != nil {
			return c.handleFetchError(url, fmt.Errorf("parse jwks: %w", err))
		}

		c.l2Set(ctx, url, raw)
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
	e.lastError = time.Now()

	if e.kf != nil {
		// Serve stale keys; extend their L1 freshness briefly so we don't refetch
		// on every request while the issuer is down.
		e.expires = time.Now().Add(c.cfg.ErrorTTL)
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
	c.l1[url] = &entry{kf: kf, expires: time.Now().Add(c.cfg.L1TTL)}
}

func (c *Cache) l2Get(ctx context.Context, url string) ([]byte, bool) {
	if c.store == nil {
		return nil, false
	}
	v, ok, err := c.store.Get(ctx, l2Key(url))
	if err != nil {
		if c.logger != nil {
			c.logger.Warn("jwks L2 get failed", zap.String("url", url), zap.Error(err))
		}
		return nil, false
	}
	return v, ok
}

func (c *Cache) l2Set(ctx context.Context, url string, raw []byte) {
	if c.store == nil {
		return
	}
	if err := c.store.Set(ctx, l2Key(url), raw, c.cfg.TTL); err != nil && c.logger != nil {
		c.logger.Warn("jwks L2 set failed", zap.String("url", url), zap.Error(err))
	}
}

func l2Key(url string) string { return "jwks:" + url }

func boolKey(b bool) string {
	if b {
		return "\x00refresh"
	}
	return ""
}

// httpFetcher fetches JWKS over HTTP with a per-request timeout.
type httpFetcher struct {
	timeout time.Duration
}

func (f *httpFetcher) Fetch(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, f.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return readAll(resp.Body, maxJWKSBytes)
}
