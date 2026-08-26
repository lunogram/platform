//go:build !enterprise

package v1

import (
	nethttp "net/http"

	"github.com/go-chi/chi/v5"
	"github.com/lunogram/platform/internal/config"
	"go.uber.org/zap"
)

// MountProxyRoutes is a no-op in OSS builds. Enterprise proxy routes
// (backoffice, courier) are only available in enterprise builds.
func MountProxyRoutes(_ *zap.Logger, _ chi.Router, _ config.Enterprise, _ func(nethttp.Handler) nethttp.Handler) {
}
