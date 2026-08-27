package verifiers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func basicRequest(t *testing.T, body any) *http.Request {
	t.Helper()
	if body == nil {
		return httptest.NewRequest(http.MethodPost, "/api/auth/login/basic/callback", nil)
	}
	encoded, err := json.Marshal(body)
	require.NoError(t, err)
	r := httptest.NewRequest(http.MethodPost, "/api/auth/login/basic/callback", bytes.NewReader(encoded))
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestBasicDriver(t *testing.T) {
	t.Parallel()
	require.Equal(t, "basic", NewBasic(config.BasicAuth{}).Driver())
}

func TestBasicVerify(t *testing.T) {
	t.Parallel()

	configured := config.BasicAuth{Email: "admin@localhost", Password: "secret"}

	tests := map[string]struct {
		config  config.BasicAuth
		body    any
		wantErr error
	}{
		"missing email": {
			config: configured, body: map[string]string{"password": "secret"},
			wantErr: ErrMissingCredentials,
		},
		"missing password": {
			config: configured, body: map[string]string{"email": "admin@localhost"},
			wantErr: ErrMissingCredentials,
		},
		"empty body": {
			config: configured, body: nil, wantErr: ErrMissingCredentials,
		},
		"invalid json": {
			config: configured, body: "not-an-object", wantErr: ErrMissingCredentials,
		},
		"wrong email": {
			config: configured, body: map[string]string{"email": "someone@else", "password": "secret"},
			wantErr: ErrInvalidCredentials,
		},
		"wrong password": {
			config: configured, body: map[string]string{"email": "admin@localhost", "password": "guess"},
			wantErr: ErrInvalidCredentials,
		},
		"driver not configured": {
			config: config.BasicAuth{}, body: map[string]string{"email": "a@b", "password": "c"},
			wantErr: ErrInvalidCredentials,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var r *http.Request
			if raw, ok := tc.body.(string); ok {
				r = httptest.NewRequest(http.MethodPost, "/api/auth/login/basic/callback", bytes.NewReader([]byte(raw)))
			} else {
				r = basicRequest(t, tc.body)
			}

			_, err := NewBasic(tc.config).Verify(context.Background(), r)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// TestBasicVerifyReturnsAVerifiedIdentity pins what the verifier is allowed to
// produce: an identity and nothing else. It is handed no ResponseWriter and no
// store, so it cannot mint a token, set a cookie, or create an admin.
func TestBasicVerifyReturnsAVerifiedIdentity(t *testing.T) {
	t.Parallel()

	verifier := NewBasic(config.BasicAuth{Email: "Admin@Localhost", Password: "secret"})

	identity, err := verifier.Verify(context.Background(), basicRequest(t, map[string]string{
		"email":    "ADMIN@localhost",
		"password": "secret",
	}))
	require.NoError(t, err)

	assert.Equal(t, BasicIssuer, identity.Issuer)
	assert.Equal(t, management.IdentityProviderBasic, identity.Provider)
	assert.Equal(t, "admin@localhost", identity.Subject,
		"the identity is keyed on the CONFIGURED address, so a case-variant submission cannot mint a second identity")
	assert.Equal(t, "admin@localhost", identity.Email)
	assert.True(t, identity.EmailVerified, "the credential IS the address")
	assert.Nil(t, identity.Actor)
}

// TestBasicHasNoWebhook records that the driver does not implement
// [auth.WebhookVerifier] at all, rather than carrying a method whose only job is
// to say "not supported".
func TestBasicHasNoWebhook(t *testing.T) {
	t.Parallel()

	var verifier any = NewBasic(config.BasicAuth{})
	_, ok := verifier.(interface {
		Webhook(context.Context, *http.Request) error
	})
	assert.False(t, ok)
}
