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
func MountProxyRoutes(logger *zap.Logger, router chi.Router, cfg config.Enterprise) {
	if cfg.Proxy.BackofficeURL != "" {
		mountServiceProxy(logger, router, "backoffice", cfg.Proxy.BackofficeURL)
	}

	if cfg.Proxy.CourierURL != "" {
		mountServiceProxy(logger, router, "courier", cfg.Proxy.CourierURL)
	}
}

// mountServiceProxy registers a reverse proxy under /{prefix}/* that strips
// the prefix and forwards requests to the given upstream URL.
func mountServiceProxy(logger *zap.Logger, router chi.Router, prefix, rawURL string) {
	remote, err := url.Parse(rawURL)
	if err != nil {
		logger.Error(fmt.Sprintf("invalid %s proxy URL, skipping", prefix), zap.String("url", rawURL), zap.Error(err))
		return
	}

	pattern := fmt.Sprintf("/%s/*", prefix)
	strip := fmt.Sprintf("/%s", prefix)

	router.Handle(pattern, nethttp.StripPrefix(strip, http.ReverseProxy(remote)))
	logger.Info(fmt.Sprintf("proxying /%s/* to %s", prefix, rawURL))
}
