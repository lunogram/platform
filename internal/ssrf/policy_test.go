package ssrf

import (
	"net"
	"testing"
)

// TestAlwaysBlocked locks the addresses that no policy may reach. The cloud
// instance metadata endpoints are the reason this set exists: 169.254.169.254
// is link-local and fd00:ec2::254 is ULA, so relaxing private addressing for an
// in-cluster receiver must not reach either.
func TestAlwaysBlocked(t *testing.T) {
	t.Parallel()

	blocked := []string{
		"169.254.169.254", // IPv4 cloud metadata (link-local)
		"fd00:ec2::254",   // IPv6 cloud metadata (ULA)
		"169.254.1.1",     // link-local unicast
		"fe80::1",         // IPv6 link-local
		"224.0.0.1",       // multicast
		"ff02::1",         // IPv6 link-local multicast
		"255.255.255.255", // IPv4 limited broadcast
		"0.0.0.0",         // unspecified
		"::",              // IPv6 unspecified
	}

	for _, policy := range []Policy{{}, {AllowPrivate: true}, {AllowHTTP: true}, {AllowPrivate: true, AllowHTTP: true}} {
		for _, raw := range blocked {
			if policy.permits(net.ParseIP(raw)) {
				t.Errorf("Policy%+v permits %s, want blocked under every policy", policy, raw)
			}
		}
	}
}

// TestAllowPrivatePermitsInternalReceivers covers the shapes AllowPrivate
// exists for: a Kubernetes ClusterIP, a sidecar on loopback, an IPv6 ULA
// service address.
func TestAllowPrivatePermitsInternalReceivers(t *testing.T) {
	t.Parallel()

	permitted := []string{"10.0.0.5", "172.16.4.1", "192.168.1.10", "127.0.0.1", "fd00::1", "100.64.0.1"}
	for _, raw := range permitted {
		ip := net.ParseIP(raw)
		if !(Policy{AllowPrivate: true}).permits(ip) {
			t.Errorf("Policy{AllowPrivate:true} blocks %s, want permitted", raw)
		}
		if (Policy{}).permits(ip) {
			t.Errorf("Policy{} permits %s, want blocked", raw)
		}
	}
}

func TestValidateURLBlocksMetadataUnderEveryPolicy(t *testing.T) {
	t.Parallel()

	for _, policy := range []Policy{{}, {AllowPrivate: true}, {AllowPrivate: true, AllowHTTP: true}} {
		for _, raw := range []string{
			"http://169.254.169.254/latest/meta-data",
			"https://169.254.169.254/latest/meta-data",
			"http://[fd00:ec2::254]/latest/meta-data",
		} {
			if err := ValidateURL(raw, policy); err == nil {
				t.Errorf("ValidateURL(%q, Policy%+v) = nil, want error", raw, policy)
			}
		}
	}
}
