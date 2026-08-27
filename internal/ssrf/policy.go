package ssrf

import (
	"fmt"
	"net"
	"net/http"
	neturl "net/url"
	"syscall"
	"time"
)

// Policy relaxes the default outbound guards for a single destination. The zero
// value is the strict policy applied to every operator-configured URL unless a
// relaxation is opted into explicitly: https only, public unicast addresses
// only, no redirect following.
//
// Each field names exactly the one guard it drops, so a config that relaxes
// something says which protection it gave up. Both are per-destination on
// purpose — a blanket relaxation would silently re-open the cloud metadata
// endpoint for every other destination in the process.
type Policy struct {
	// AllowPrivate permits loopback, RFC 1918, IPv6 ULA and CGNAT destinations.
	// Required for receivers that live inside the deployment's own network (a
	// Kubernetes ClusterIP, a sidecar).
	//
	// It does not permit link-local addresses or the cloud instance metadata
	// endpoints, which stay blocked under every policy — see [Policy.permits].
	AllowPrivate bool
	// AllowHTTP permits plaintext http:// destinations. Only sensible together
	// with AllowPrivate, for an in-cluster receiver that terminates TLS at the
	// mesh rather than at the pod.
	AllowHTTP bool
}

// ValidateURL reports whether raw is safe to configure as an outbound
// destination under p. It is the cheap up-front guard run at config-load time;
// the authoritative protection against DNS rebinding is the dialer in
// [PolicyHTTPClient], which re-checks the resolved IP at connection time.
func ValidateURL(raw string, p Policy) error {
	u, err := neturl.Parse(raw)
	if err != nil {
		return fmt.Errorf("ssrf: invalid url: %w", err)
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !p.AllowHTTP {
			return fmt.Errorf("ssrf: url must use https")
		}
	default:
		return fmt.Errorf("ssrf: url must use https")
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("ssrf: url must have a host")
	}
	if ip := net.ParseIP(host); ip != nil && !p.permits(ip) {
		return fmt.Errorf("ssrf: url host is not a public address")
	}
	return nil
}

// metadataIPv6 is the IPv6 cloud instance metadata endpoint (fd00:ec2::254).
// It sits inside the ULA range, so unlike its IPv4 counterpart it is not caught
// by the link-local check and has to be named.
var metadataIPv6 = net.ParseIP("fd00:ec2::254")

// alwaysBlocked reports whether ip may never be dialed, whatever the policy.
//
// Link-local unicast is blocked here rather than left to AllowPrivate because
// the one address that matters in that range — 169.254.169.254, the cloud
// instance metadata endpoint — is the highest-value SSRF target there is, and
// no plausible webhook receiver has a link-local address. AllowPrivate exists
// for receivers inside the deployment's own network, and those are RFC 1918 or
// ULA, not link-local. Multicast, broadcast and the unspecified address are
// never a meaningful destination for an outbound request at all.
func alwaysBlocked(ip net.IP) bool {
	return ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() ||
		ip.Equal(net.IPv4bcast) || ip.Equal(metadataIPv6)
}

// permits reports whether ip may be dialed under p.
func (p Policy) permits(ip net.IP) bool {
	if alwaysBlocked(ip) {
		return false
	}
	if p.AllowPrivate {
		return true
	}
	return isPublicIP(ip)
}

// PolicyHTTPClient returns a client that refuses to connect to addresses
// disallowed by p and does not follow redirects. The IP is checked at dial time
// — after DNS resolution — which closes the DNS-rebinding gap a URL-string
// check alone cannot.
func PolicyHTTPClient(timeout time.Duration, p Policy) *http.Client {
	dialer := &net.Dialer{
		Timeout: timeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || !p.permits(ip) {
				return fmt.Errorf("ssrf: refusing to connect to disallowed address %q", host)
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
