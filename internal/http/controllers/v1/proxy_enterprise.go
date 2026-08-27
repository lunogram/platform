//go:build enterprise

package v1

import (
	"fmt"
	nethttp "net/http"
	"net/url"

	"github.com/go-chi/chi/v5"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/http"
	"go.uber.org/zap"
)

// MountProxyRoutes registers reverse proxy routes for enterprise services
// (backoffice, courier) on the given router. Routes are only mounted when
// the corresponding PROXY_*_URL environment variable is configured.
//
// These routes sit on the root router, outside the OpenAPI validator and so
// outside its AuthenticationFunc, and both upstreams serve whatever the proxy
// forwards without authenticating it themselves. authenticate is therefore the
// only credential check standing in front of them, and it is not optional:
// every mounted route is wrapped in it.
func MountProxyRoutes(logger *zap.Logger, router chi.Router, cfg config.Enterprise, authenticate func(nethttp.Handler) nethttp.Handler) {
	if cfg.Proxy.BackofficeURL != "" {
		mountServiceProxy(logger, router, "backoffice", cfg.Proxy.BackofficeURL, authenticate)
	}

	if cfg.Proxy.CourierURL != "" {
		mountServiceProxy(logger, router, "courier", cfg.Proxy.CourierURL, authenticate)
	}
}

// mountServiceProxy registers an authenticated reverse proxy under /{prefix}/*
// that strips the prefix and forwards requests to the given upstream URL.
//
// The chain runs authenticate → forward identity → strip prefix → proxy. The
// identity headers describe the actor authentication resolved, so attaching them
// has to come second. Nothing in the chain wraps the response writer, which is
// what keeps the upstream's flushes reaching the client — the AI builder streams
// its replies through here.
func mountServiceProxy(logger *zap.Logger, router chi.Router, prefix, rawURL string, authenticate func(nethttp.Handler) nethttp.Handler) {
	remote, err := url.Parse(rawURL)
	if err != nil {
		logger.Error(fmt.Sprintf("invalid %s proxy URL, skipping", prefix), zap.String("url", rawURL), zap.Error(err))
		return
	}

	pattern := fmt.Sprintf("/%s/*", prefix)
	strip := fmt.Sprintf("/%s", prefix)

	proxy := nethttp.StripPrefix(strip, http.ReverseProxy(remote))
	router.Handle(pattern, authenticate(http.ForwardActorIdentity(proxy)))
	logger.Info(fmt.Sprintf("proxying /%s/* to %s", prefix, rawURL))
}
