package outbound

import (
	"net/http"
	"time"

	"github.com/lunogram/platform/internal/ssrf"
)

// ssrfClient builds the guarded HTTP client for a destination.
func ssrfClient(timeout time.Duration, network Network) *http.Client {
	return ssrf.PolicyHTTPClient(timeout, network.Policy())
}

// ValidateURL checks a configured destination URL against its network policy.
// It is exported so configuration loaders can reject a bad URL at boot rather
// than at first use.
func ValidateURL(raw string, network Network) error {
	return ssrf.ValidateURL(raw, network.Policy())
}
