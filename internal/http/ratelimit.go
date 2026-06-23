package http

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/ratelimit"
	"github.com/lunogram/platform/internal/rbac"
)

// RateLimit limits requests to limit per window. It keys on the authenticated
// auth method, so a key gets a single budget across every endpoint it can call
// — client and management alike — and falls back to the client IP for
// unauthenticated traffic. It must run after authentication so the actor is
// available on the context.
//
// writeProblem renders the 429 response in the calling surface's problem shape.
//
// trustedProxyHops is the number of reverse proxies in front of the server; it
// controls how the client IP is derived from X-Forwarded-For (see clientIP).
//
// The limiter fails open on Redis errors (see ratelimit.Limiter), so an
// unavailable Redis never blocks legitimate traffic.
func RateLimit(limiter *ratelimit.Limiter, limit int, window time.Duration, trustedProxyHops int, writeProblem func(http.ResponseWriter, error)) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			allowed, retryAfter, _ := limiter.Allow(r.Context(), rateLimitKey(r, trustedProxyHops), limit, window)
			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
				writeProblem(w, problem.ErrTooManyRequests())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// rateLimitKey identifies the rate-limit bucket for a request: the auth method
// id when authenticated, otherwise the client IP. The bucket is surface-
// agnostic ("key:"/"ip:", not per-surface) so a key shares one budget across
// the client and management APIs.
func rateLimitKey(r *http.Request, trustedProxyHops int) string {
	if actor := rbac.FromContext(r.Context()); actor != nil && actor.ID != "" {
		// Verified end users (session / trusted issuer) all share their auth
		// method's id as the actor id, so partition the bucket by the verified
		// subject; otherwise one end user could exhaust the whole method's budget.
		if actor.Subject != "" {
			return "key:" + actor.ID + ":" + actor.Subject
		}
		return "key:" + actor.ID
	}
	return "ip:" + clientIP(r, trustedProxyHops)
}

// clientIP returns the originating client IP. X-Forwarded-For is attacker-
// controlled, so it is only trusted up to trustedProxyHops entries (the reverse
// proxies the operator runs): the client IP is the hop immediately to the left
// of those trusted proxies in the chain [XFF..., RemoteAddr]. When
// trustedProxyHops is 0 the header is ignored entirely and the connection's
// remote address is used.
func clientIP(r *http.Request, trustedProxyHops int) string {
	remote := r.RemoteAddr
	if host, _, err := net.SplitHostPort(remote); err == nil {
		remote = host
	}
	if trustedProxyHops <= 0 {
		return remote
	}

	// Chain runs originator → … → closest proxy: the X-Forwarded-For entries in
	// order, then the direct peer (the right-most, most-trusted hop).
	var chain []string
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		for _, hop := range strings.Split(fwd, ",") {
			if hop = strings.TrimSpace(hop); hop != "" {
				chain = append(chain, hop)
			}
		}
	}
	chain = append(chain, remote)

	// Strip the trusted proxies from the right; the next hop is the client.
	idx := len(chain) - 1 - trustedProxyHops
	if idx < 0 {
		idx = 0
	}
	return chain[idx]
}
