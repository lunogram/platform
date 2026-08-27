package http

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/lunogram/platform/internal/rbac"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// Identity headers describing the authenticated caller, added by
// [ForwardActorIdentity] on the proxy hop. They are named so an upstream service
// can attribute a proxied request without re-verifying the credential.
//
// An upstream may only trust these on a connection it accepts exclusively from
// the proxy — they say nothing about a request that reached the service by
// another route.
const (
	HeaderActorID        = "X-Lunogram-Actor-Id"
	HeaderOrganizationID = "X-Lunogram-Organization-Id"
)

// forwardedIdentityHeaders lists every header [ForwardActorIdentity] owns. All of
// them are cleared before any are set, so a header the client sent itself can
// never survive into the upstream request.
var forwardedIdentityHeaders = []string{HeaderActorID, HeaderOrganizationID}

// ForwardActorIdentity replaces the identity headers on a proxied request with
// the authenticated actor resolved by the authentication middleware, so an
// upstream service that trusts only the proxy can attribute the request.
//
// The headers are cleared unconditionally — including when no actor is present —
// because the values are a statement by this server about who it authenticated,
// and a client that sends its own copy is claiming an identity it was not
// granted. Clearing first also means a route that forgets its authentication
// middleware forwards no identity at all rather than the client's own.
func ForwardActorIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, header := range forwardedIdentityHeaders {
			r.Header.Del(header)
		}
		unlistAsHopByHop(r.Header, forwardedIdentityHeaders)

		if actor := rbac.FromContext(r.Context()); actor != nil {
			r.Header.Set(HeaderActorID, actor.ID)
			r.Header.Set(HeaderOrganizationID, actor.OrganizationID.String())
		}

		next.ServeHTTP(w, r)
	})
}

// unlistAsHopByHop drops the given header names from the request's Connection
// header. A client can name any header there, and the reverse proxy honours that
// list by deleting those headers from the upstream request — which would let a
// caller suppress the identity we are about to attach. Only our own names are
// removed, so a genuine "Connection: Upgrade" still reaches the upstream.
func unlistAsHopByHop(header http.Header, names []string) {
	values := header.Values("Connection")
	if len(values) == 0 {
		return
	}

	kept := make([]string, 0, len(values))
	for _, value := range values {
		for _, token := range strings.Split(value, ",") {
			token = strings.TrimSpace(token)
			if token == "" || slices.ContainsFunc(names, func(name string) bool { return strings.EqualFold(name, token) }) {
				continue
			}
			kept = append(kept, token)
		}
	}

	header.Del("Connection")
	if len(kept) > 0 {
		header.Set("Connection", strings.Join(kept, ", "))
	}
}

// ReverseProxy creates a reverse proxy handler that forwards requests to the specified remote URL.
// It injects OpenTelemetry context propagation headers into the requests and supports
// streaming responses (e.g. Server-Sent Events) by flushing data immediately.
func ReverseProxy(remote *url.URL) http.Handler {
	propagator := otel.GetTextMapPropagator()

	proxy := httputil.NewSingleHostReverseProxy(remote)
	proxy.FlushInterval = -1 * time.Millisecond // flush immediately for SSE/streaming

	director := proxy.Director
	proxy.Director = func(req *http.Request) {
		propagator.Inject(req.Context(), propagation.HeaderCarrier(req.Header))
		director(req)
	}

	return otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Host = remote.Host
		r.URL.Scheme = remote.Scheme
		r.Host = remote.Host
		proxy.ServeHTTP(w, r)
	}), "reverse-proxy")
}
