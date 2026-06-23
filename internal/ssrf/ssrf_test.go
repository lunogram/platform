package ssrf

import "testing"

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
