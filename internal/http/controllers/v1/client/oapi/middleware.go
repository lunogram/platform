package oapi

import (
	"errors"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/cors"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/ratelimit"
	"github.com/lunogram/platform/internal/rbac"
	middleware "github.com/oapi-codegen/nethttp-middleware"
)

func Validator(spec *openapi3.T, options openapi3filter.Options) func(next http.Handler) http.Handler {
	return middleware.OapiRequestValidatorWithOptions(spec, &middleware.Options{
		Options:              options,
		DoNotValidateServers: true,
		ErrorHandler: func(w http.ResponseWriter, message string, statusCode int) {
			err := problem.WithStatus(problem.WithDescription(errors.New(message), "bad request", message), statusCode)
			WriteProblem(w, err)
		},
	})
}

// CORS configures cross-origin access for the client API. The client API is
// strictly bearer-authenticated (no cookies), so credentials are disabled and a
// wildcard origin is safe: a bearer token is never sent ambiently by the
// browser, so a wildcard cannot be abused the way it can with cookie auth.
//
// Public keys are intentionally usable from any origin (they are embedded in
// browser apps, like a publishable key); they are constrained by being
// write-only, event-allow-listed and own-data scoped, not by origin. There is
// deliberately no per-policy origin allow-list.
func CORS() func(next http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	})
}

// RateLimit limits client API requests to limit requests per window. It keys on
// the authenticated access policy when present (so each key gets its own
// budget) and falls back to the client IP for unauthenticated traffic. It must
// run after authentication so the actor is available on the context.
//
// trustedProxyHops is the number of reverse proxies in front of the server; it
// controls how the client IP is derived from X-Forwarded-For (see clientIP).
//
// The limiter fails open on Redis errors (see ratelimit.Limiter), so an
// unavailable Redis never blocks legitimate traffic.
func RateLimit(limiter *ratelimit.Limiter, limit int, window time.Duration, trustedProxyHops int) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			allowed, retryAfter, _ := limiter.Allow(r.Context(), rateLimitKey(r, trustedProxyHops), limit, window)
			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
				WriteProblem(w, problem.ErrTooManyRequests())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// rateLimitKey identifies the rate-limit bucket for a request: the access policy
// id when authenticated, otherwise the client IP.
func rateLimitKey(r *http.Request, trustedProxyHops int) string {
	if actor := rbac.FromContext(r.Context()); actor != nil && actor.ID != "" {
		return "client:key:" + actor.ID
	}
	return "client:ip:" + clientIP(r, trustedProxyHops)
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
