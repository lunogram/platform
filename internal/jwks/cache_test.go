package jwks

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/lunogram/platform/internal/container"
	iredis "github.com/lunogram/platform/internal/redis"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testJWKS builds a valid JWKS document containing a single RSA public key.
func testJWKS(t *testing.T, kid string) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	jwk, err := jwkset.NewJWKFromKey(key.Public(), jwkset.JWKOptions{
		Metadata: jwkset.JWKMetadataOptions{KID: kid},
	})
	require.NoError(t, err)

	storage := jwkset.NewMemoryStorage()
	require.NoError(t, storage.KeyWrite(context.Background(), jwk))
	raw, err := storage.JSONPublic(context.Background())
	require.NoError(t, err)
	return raw
}

// fakeFetcher returns a fixed document and counts calls. An err, when set, is
// returned instead.
type fakeFetcher struct {
	raw   []byte
	calls atomic.Int32
	mu    sync.Mutex
	err   error
}

func (f *fakeFetcher) Fetch(_ context.Context, _ string) ([]byte, error) {
	f.calls.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return f.raw, nil
}

func (f *fakeFetcher) setErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

const url = "https://issuer.example/.well-known/jwks.json"

func TestCacheServesFromL1(t *testing.T) {
	t.Parallel()
	f := &fakeFetcher{raw: testJWKS(t, "k1")}
	c := New(Config{}, nil, f, nil)

	for i := 0; i < 3; i++ {
		kf, err := c.Keyfunc(context.Background(), url)
		require.NoError(t, err)
		require.NotNil(t, kf)
	}
	assert.Equal(t, int32(1), f.calls.Load(), "subsequent calls should hit L1")
}

func TestCacheSharesViaL2(t *testing.T) {
	t.Parallel()
	client := testRedis(t)
	// Distinct cache instances over the same Redis (same prefix) stand in for two
	// processes sharing the L2.
	prefix := fmt.Sprintf("jwkstest:%d:", time.Now().UnixNano())
	l2a := iredis.NewCache[json.RawMessage](client, prefix, time.Hour, nil)
	l2b := iredis.NewCache[json.RawMessage](client, prefix, time.Hour, nil)

	f1 := &fakeFetcher{raw: testJWKS(t, "k1")}
	f2 := &fakeFetcher{raw: testJWKS(t, "k2")}

	// First instance fetches and populates the shared L2.
	c1 := New(Config{}, l2a, f1, nil)
	_, err := c1.Keyfunc(context.Background(), url)
	require.NoError(t, err)
	assert.Equal(t, int32(1), f1.calls.Load())

	// Second instance (cold L1) resolves from L2 without fetching.
	c2 := New(Config{}, l2b, f2, nil)
	_, err = c2.Keyfunc(context.Background(), url)
	require.NoError(t, err)
	assert.Equal(t, int32(0), f2.calls.Load(), "L2 hit should avoid an issuer fetch")
}

func TestCacheCoalescesConcurrentFetches(t *testing.T) {
	t.Parallel()
	f := &fakeFetcher{raw: testJWKS(t, "k1")}
	c := New(Config{}, nil, f, nil)

	var wg sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = c.Keyfunc(context.Background(), url)
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(1), f.calls.Load(), "concurrent cold calls should coalesce into one fetch")
}

func TestCacheFailsOpenToLastKnownGood(t *testing.T) {
	t.Parallel()
	f := &fakeFetcher{raw: testJWKS(t, "k1")}
	// Tiny L1 TTL so the second call re-resolves; no L2 so it reaches the fetcher.
	c := New(Config{L1TTL: time.Millisecond}, nil, f, nil)

	_, err := c.Keyfunc(context.Background(), url)
	require.NoError(t, err)

	time.Sleep(5 * time.Millisecond)
	f.setErr(errors.New("issuer down"))

	kf, err := c.Keyfunc(context.Background(), url)
	require.NoError(t, err, "should serve stale keys when the issuer is unreachable")
	require.NotNil(t, kf)
	assert.Equal(t, int32(2), f.calls.Load())
}

func TestCacheNoKeysFetchErrorReturnsError(t *testing.T) {
	t.Parallel()
	f := &fakeFetcher{err: errors.New("issuer down")}
	c := New(Config{}, nil, f, nil)

	_, err := c.Keyfunc(context.Background(), url)
	require.Error(t, err, "with no cached keys a fetch failure must surface")
}

func TestCacheRefreshForcesFetch(t *testing.T) {
	t.Parallel()
	f := &fakeFetcher{raw: testJWKS(t, "k1")}
	c := New(Config{}, nil, f, nil)

	_, err := c.Keyfunc(context.Background(), url)
	require.NoError(t, err)
	assert.Equal(t, int32(1), f.calls.Load())

	// Refresh bypasses the freshness window (used on an unknown key id).
	_, err = c.Refresh(context.Background(), url)
	require.NoError(t, err)
	assert.Equal(t, int32(2), f.calls.Load())
}

func TestReadAllRejectsOversizedDocument(t *testing.T) {
	t.Parallel()
	// At the limit is accepted.
	at, err := readAll(bytes.NewReader(make([]byte, maxJWKSBytes)), maxJWKSBytes)
	require.NoError(t, err)
	require.Len(t, at, maxJWKSBytes)

	// One byte over is rejected.
	_, err = readAll(bytes.NewReader(make([]byte, maxJWKSBytes+1)), maxJWKSBytes)
	require.Error(t, err)
}

// testRedis starts (or reuses) a Redis test container and returns a client.
func testRedis(t *testing.T) *goredis.Client {
	t.Helper()
	opts, err := goredis.ParseURL(container.RunRedis(t))
	require.NoError(t, err)
	client := goredis.NewClient(opts)
	t.Cleanup(func() { _ = client.Close() })
	return client
}
