package http

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

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
