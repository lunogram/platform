// Package ssrf provides SSRF guards for outbound requests to operator-supplied
// URLs (e.g. a trusted issuer's JWKS endpoint): an up-front URL validator and an
// HTTP client that refuses to connect to non-public addresses at dial time.
package ssrf

import (
	"fmt"
	"net"
	"net/http"
	neturl "net/url"
	"syscall"
	"time"
)

// ValidateSourceURL reports whether raw is safe to register as an outbound
// source: it must be an absolute https URL whose host is not an obviously-
// internal address. It is a cheap up-front guard run when a URL is configured.
// The authoritative protection against DNS rebinding is the dialer in
// [SafeHTTPClient], which re-checks the resolved IP at connection time.
func ValidateSourceURL(raw string) error {
	u, err := neturl.Parse(raw)
	if err != nil {
		return fmt.Errorf("ssrf: invalid url: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("ssrf: url must use https")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("ssrf: url must have a host")
	}
	if ip := net.ParseIP(host); ip != nil && !isPublicIP(ip) {
		return fmt.Errorf("ssrf: url host is not a public address")
	}
	return nil
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
	dialer := &net.Dialer{
		Timeout: timeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || !isPublicIP(ip) {
				return fmt.Errorf("ssrf: refusing to connect to non-public address %q", host)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   timeout,
			ResponseHeaderTimeout: timeout,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse // do not follow redirects to other hosts
		},
	}
}
