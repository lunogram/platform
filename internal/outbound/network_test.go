package outbound

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateURLStrictByDefault(t *testing.T) {
	t.Parallel()

	strict := Network{}
	for _, raw := range []string{
		"http://receiver.example.com/hook", // plaintext
		"https://127.0.0.1/hook",           // loopback
		"https://10.0.0.5/hook",            // RFC 1918
		"https://169.254.169.254/hook",     // cloud metadata
		"https://[fd00::1]/hook",           // IPv6 ULA
		"https://100.64.0.1/hook",          // CGNAT
		"ftp://receiver.example.com/hook",  // wrong scheme
		"receiver.example.com/hook",        // no scheme
	} {
		assert.Error(t, ValidateURL(raw, strict), "expected %q to be rejected under the strict policy", raw)
	}

	assert.NoError(t, ValidateURL("https://receiver.example.com/hook", strict))
}

func TestValidateURLRelaxations(t *testing.T) {
	t.Parallel()

	// Each relaxation drops exactly the guard it names, and no more.
	assert.NoError(t, ValidateURL("https://10.0.0.5/hook", Network{AllowPrivate: true}))
	assert.Error(t, ValidateURL("http://10.0.0.5/hook", Network{AllowPrivate: true}),
		"allow_private must not also permit plaintext")

	assert.NoError(t, ValidateURL("http://receiver.example.com/hook", Network{AllowHTTP: true}))
	assert.Error(t, ValidateURL("http://10.0.0.5/hook", Network{AllowHTTP: true}),
		"allow_http must not also permit private addresses")

	assert.NoError(t, ValidateURL("http://10.0.0.5/hook", Network{AllowPrivate: true, AllowHTTP: true}))
}

// TestCloudMetadataIsBlockedUnderEveryPolicy is the guard that matters most:
// relaxing the network policy for an in-cluster receiver must never open a path
// to the instance metadata endpoint.
func TestCloudMetadataIsBlockedUnderEveryPolicy(t *testing.T) {
	t.Parallel()

	for _, network := range []Network{{}, {AllowPrivate: true}, {AllowPrivate: true, AllowHTTP: true}} {
		for _, raw := range []string{
			"https://169.254.169.254/latest/meta-data",
			"http://169.254.169.254/latest/meta-data",
			"http://[fd00:ec2::254]/latest/meta-data",
			"http://169.254.1.1/hook",
		} {
			assert.Error(t, ValidateURL(raw, network),
				"%q must be refused under %+v", raw, network)
		}
	}
}

func TestClientRefusesPrivateAddressAtDialTime(t *testing.T) {
	t.Parallel()

	// The listener is on loopback, so the strict dialer must refuse it even
	// though the URL string was never checked here — this is the guard that
	// survives a DNS name resolving to a private address.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(Options{
		Timeout: time.Second,
		Network: Network{AllowHTTP: true},
		Retry:   Retry{MaxAttempts: 1, InitialInterval: time.Millisecond, MaxInterval: time.Millisecond},
	})
	require.NoError(t, err)

	_, err = client.Do(t.Context(), Request{Method: http.MethodGet, URL: server.URL})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "refusing to connect")
}

func TestNetworkRelaxedReporting(t *testing.T) {
	t.Parallel()

	assert.False(t, Network{}.Relaxed())
	assert.True(t, Network{AllowPrivate: true}.Relaxed())
	assert.True(t, Network{AllowHTTP: true}.Relaxed())
}
