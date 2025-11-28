package http

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// ReverseProxy creates a reverse proxy handler that forwards requests to the specified remote URL.
// It also injects OpenTelemetry context propagation headers into the requests.
func ReverseProxy(remote *url.URL) http.Handler {
	propagator := otel.GetTextMapPropagator()

	proxy := httputil.NewSingleHostReverseProxy(remote)
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
