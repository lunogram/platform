package ssrf

import (
	"net"
	"testing"
)

// TestIsPublicIP locks the per-range classification, including the IPv4 limited
// broadcast address (255.255.255.255) the hardening added to the block list.
func TestIsPublicIP(t *testing.T) {
	t.Parallel()

	public := []string{"8.8.8.8", "1.1.1.1", "203.0.113.10", "2606:4700:4700::1111"}
	for _, s := range public {
		if !isPublicIP(net.ParseIP(s)) {
			t.Errorf("isPublicIP(%s) = false, want true", s)
		}
	}

	blocked := []string{
		"255.255.255.255", // IPv4 limited broadcast
		"0.0.0.0",         // unspecified
		"127.0.0.1",       // loopback
		"169.254.169.254", // link-local (cloud metadata)
		"10.0.0.1",        // RFC 1918
		"192.168.0.1",     // RFC 1918
		"172.16.0.1",      // RFC 1918
		"100.64.0.1",      // CGNAT
		"224.0.0.1",       // multicast
		"::1",             // IPv6 loopback
		"fd00::1",         // IPv6 ULA
		"fe80::1",         // IPv6 link-local
	}
	for _, s := range blocked {
		if isPublicIP(net.ParseIP(s)) {
			t.Errorf("isPublicIP(%s) = true, want false", s)
		}
	}
}

func TestValidateSourceURL(t *testing.T) {
	t.Parallel()

	valid := []string{
		"https://idp.example.com/.well-known/jwks.json",
		"https://login.microsoftonline.com/common/discovery/keys",
	}
	for _, u := range valid {
		if err := ValidateSourceURL(u); err != nil {
			t.Errorf("ValidateSourceURL(%q) = %v, want nil", u, err)
		}
	}

	invalid := []string{
		"http://idp.example.com/jwks.json",         // not https
		"ftp://idp.example.com/jwks.json",          // wrong scheme
		"https://",                                 // no host
		"https://169.254.169.254/latest/meta-data", // cloud metadata (link-local)
		"https://127.0.0.1/jwks.json",              // loopback
		"https://10.0.0.5/jwks.json",               // RFC 1918
		"https://192.168.1.1/jwks.json",            // RFC 1918
		"https://100.64.0.1/jwks.json",             // CGNAT
		"https://255.255.255.255/jwks.json",        // IPv4 limited broadcast
		"https://0.0.0.0/jwks.json",                // unspecified
		"https://[::1]/jwks.json",                  // IPv6 loopback
		"https://[fd00::1]/jwks.json",              // IPv6 ULA
		"not-a-url",                                // unparseable / no scheme
	}
	for _, u := range invalid {
		if err := ValidateSourceURL(u); err == nil {
			t.Errorf("ValidateSourceURL(%q) = nil, want error", u)
		}
	}
}
