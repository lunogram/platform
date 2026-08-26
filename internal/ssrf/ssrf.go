// Package ssrf provides SSRF guards for outbound requests to operator-supplied
// URLs (e.g. a trusted issuer's JWKS endpoint, a configured webhook receiver):
// an up-front URL validator and an HTTP client that refuses to connect to
// non-public addresses at dial time.
//
// The strict behaviour is the default. Destinations that legitimately live
// inside the deployment's own network opt out of individual guards through
// [Policy]; see [ValidateURL] and [PolicyHTTPClient].
package ssrf

import (
	"net"
	"net/http"
	"time"
)

// ValidateSourceURL reports whether raw is safe to register as an outbound
// source: it must be an absolute https URL whose host is not an obviously-
// internal address. It is a cheap up-front guard run when a URL is configured.
// The authoritative protection against DNS rebinding is the dialer in
// [SafeHTTPClient], which re-checks the resolved IP at connection time.
func ValidateSourceURL(raw string) error {
	return ValidateURL(raw, Policy{})
}

// isPublicIP reports whether ip is a globally-routable unicast address, i.e. not
// loopback, private (RFC 1918 / ULA fc00::/7), link-local (incl. the
// 169.254.169.254 cloud metadata endpoint), CGNAT, multicast, broadcast or
// unspecified.
func isPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() ||
		ip.Equal(net.IPv4bcast) {
		return false
	}
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1]&0xc0 == 0x40 {
		return false // 100.64.0.0/10 (RFC 6598 carrier-grade NAT)
	}
	return true
}

// SafeHTTPClient returns a client that refuses to connect to non-public
// addresses and does not follow redirects. The IP is checked at dial time —
// after DNS resolution — which closes the DNS-rebinding gap a URL-string check
// alone cannot.
func SafeHTTPClient(timeout time.Duration) *http.Client {
	return PolicyHTTPClient(timeout, Policy{})
}
