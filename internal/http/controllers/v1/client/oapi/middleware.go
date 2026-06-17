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
// Per-policy origin allow-listing is enforced after authentication, against the
// resolved access policy, rather than here.
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
// The limiter fails open on Redis errors (see ratelimit.Limiter), so an
// unavailable Redis never blocks legitimate traffic.
func RateLimit(limiter *ratelimit.Limiter, limit int, window time.Duration) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			allowed, retryAfter, _ := limiter.Allow(r.Context(), rateLimitKey(r), limit, window)
			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
				WriteProblem(w, problem.ErrTooManyRequests())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// rateLimitKey identifies the rate-limit bucket for a request: the access
// policy id when authenticated, otherwise the client IP.
func rateLimitKey(r *http.Request) string {
	if actor := rbac.FromContext(r.Context()); actor != nil && actor.ID != "" {
		return "client:key:" + actor.ID
	}
	return "client:ip:" + clientIP(r)
}

// clientIP returns the originating client IP, preferring the first hop in
// X-Forwarded-For and falling back to the connection's remote address.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if first := strings.TrimSpace(strings.SplitN(fwd, ",", 2)[0]); first != "" {
			return first
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
