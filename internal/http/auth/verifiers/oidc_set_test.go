package verifiers

import (
	"testing"

	"github.com/lunogram/platform/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Every provider is validated at boot rather than at its first login. A
// deployment offering four ways in should not discover on a Monday morning that
// one of them was never going to work.
func TestNewOIDCSet(t *testing.T) {
	t.Parallel()

	opts := testOIDCOptions(t, config.OIDCProvider{})
	usable := func(id, issuer string) config.OIDCProvider {
		return config.OIDCProvider{ID: id, Issuer: issuer, ClientID: "c", ClientSecret: "s"}
	}

	t.Run("providers keep declaration order", func(t *testing.T) {
		set, err := NewOIDCSet(config.OIDCAuth{Providers: []config.OIDCProvider{
			usable("okta", "https://okta.test"),
			usable("entra", "https://entra.test"),
		}}, opts)
		require.NoError(t, err)

		all := set.All()
		require.Len(t, all, 2)
		assert.Equal(t, "okta", all[0].ID())
		assert.Equal(t, "entra", all[1].ID())
		assert.Nil(t, set.Only(), "there is nothing to pick by default when there are two")
	})

	t.Run("the name falls back to the id", func(t *testing.T) {
		set, err := NewOIDCSet(config.OIDCAuth{Providers: []config.OIDCProvider{
			usable("okta", "https://okta.test"),
		}}, opts)
		require.NoError(t, err)
		assert.Equal(t, "okta", set.Provider("okta").Name())
		assert.NotNil(t, set.Only())
	})

	t.Run("an id nobody declared resolves to nothing", func(t *testing.T) {
		set, err := NewOIDCSet(config.OIDCAuth{Providers: []config.OIDCProvider{
			usable("okta", "https://okta.test"),
		}}, opts)
		require.NoError(t, err)
		assert.Nil(t, set.Provider("entra"))
	})

	// The id is half of a login URL and of the redirect URI the operator
	// registers, so it has to survive a path segment.
	t.Run("an id that is not usable in a URL is refused", func(t *testing.T) {
		_, err := NewOIDCSet(config.OIDCAuth{Providers: []config.OIDCProvider{
			usable("okta/staff", "https://okta.test"),
		}}, opts)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "URL path")
	})

	// PathEscape leaves these alone, but a browser resolves them away: the
	// callback URL an operator registered would point somewhere this deployment
	// does not serve, and the login could never complete.
	t.Run("a dot-segment id is refused", func(t *testing.T) {
		for _, id := range []string{".", ".."} {
			_, err := NewOIDCSet(config.OIDCAuth{Providers: []config.OIDCProvider{
				usable(id, "https://okta.test"),
			}}, opts)
			require.Error(t, err, id)
			assert.Contains(t, err.Error(), "resolves away")
		}
	})

	t.Run("a provider with no id is refused", func(t *testing.T) {
		_, err := NewOIDCSet(config.OIDCAuth{Providers: []config.OIDCProvider{
			usable("", "https://okta.test"),
		}}, opts)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no id")
	})

	// Two providers under one id would make the login URL ambiguous, and the
	// second would silently never be reachable.
	t.Run("two providers sharing an id are refused", func(t *testing.T) {
		_, err := NewOIDCSet(config.OIDCAuth{Providers: []config.OIDCProvider{
			usable("okta", "https://okta.test"),
			usable("okta", "https://other.test"),
		}}, opts)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "share the id")
	})

	t.Run("a provider that cannot be built names itself", func(t *testing.T) {
		_, err := NewOIDCSet(config.OIDCAuth{Providers: []config.OIDCProvider{
			usable("okta", "https://okta.test"),
			{ID: "entra", Issuer: "https://entra.test"},
		}}, opts)
		require.ErrorIs(t, err, ErrOIDCNotConfigured)
		assert.Contains(t, err.Error(), "entra")
	})

	t.Run("naming the driver with no providers at all is refused", func(t *testing.T) {
		_, err := NewOIDCSet(config.OIDCAuth{}, opts)
		require.ErrorIs(t, err, ErrOIDCNotConfigured)
	})
}
