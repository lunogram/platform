package config

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/lunogram/platform/internal/mailer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultsMatchTheirFormerTags guards the move of every default out of an
// envDefault struct tag and into [Defaults].
//
// testdata/defaults.json was captured by running env.Parse over an empty
// environment while the tags were still in place, so it records what a
// deployment configuring nothing used to get. A default that shifted during the
// move is a behaviour change nobody asked for and nothing else would catch: the
// compiler is happy either way, and the setting only surfaces in production.
//
// auth.password.registration moved to auth.basic.registration when the two
// local-credential drivers became one; the key moves in the file and its value
// does not, which is exactly the guarantee this test exists to make.
//
// metrics_address postdates the capture: it arrived with the endpoint that
// serves the registry, and its entry in the file records the envDefault tag it
// was introduced with, so the setting is held to the same guarantee as the rest.
//
// The mail section is excluded because it is deliberately redesigned in this
// change -- a channel replaced a bare host, and the sender moved out of the
// transport -- and is asserted separately below. auth.oidc is excluded for the
// opposite reason: the driver did not exist when the snapshot was taken, so it
// has no former tags to match and is pinned separately too. The captured auth.JWKS was
// dropped from the file because the type marshals to an object but unmarshals
// from a string; it holds no default either way.
func TestDefaultsMatchTheirFormerTags(t *testing.T) {
	raw, err := os.ReadFile("testdata/defaults.json")
	require.NoError(t, err)

	var former Node
	require.NoError(t, json.Unmarshal(raw, &former))

	current := Defaults()

	former.Mail = mailer.Config{}
	current.Mail = mailer.Config{}
	current.Auth.OIDC = OIDCAuth{}

	assert.Equal(t, former, current)
}

// The single-provider form and the list are two ways to say the same thing, and
// a deployment picks one. Mixing them is refused rather than merged, because
// there is no order in which one obviously wins.
func TestOIDCProviderForms(t *testing.T) {
	t.Parallel()

	t.Run("nothing configured resolves to nothing", func(t *testing.T) {
		providers, err := Defaults().Auth.OIDC.Resolve()
		require.NoError(t, err)
		assert.Empty(t, providers)
		assert.False(t, Defaults().Auth.OIDC.Configured())
	})

	t.Run("the environment form becomes one provider", func(t *testing.T) {
		cfg := OIDCAuth{Provider: OIDCProvider{Issuer: "https://idp.test", ClientID: "c", ClientSecret: "s"}}
		providers, err := cfg.Resolve()
		require.NoError(t, err)
		require.Len(t, providers, 1)
		assert.Equal(t, DefaultOIDCProviderID, providers[0].ID,
			"the id appears in the redirect URI an operator registers, so it has to be stable")
	})

	t.Run("the list is taken in declaration order", func(t *testing.T) {
		cfg := OIDCAuth{Providers: []OIDCProvider{
			{ID: "okta", Issuer: "https://okta.test"},
			{ID: "entra", Issuer: "https://entra.test"},
		}}
		providers, err := cfg.Resolve()
		require.NoError(t, err)
		require.Len(t, providers, 2)
		assert.Equal(t, "okta", providers[0].ID)
		assert.Equal(t, "entra", providers[1].ID)
	})

	// Any inline field counts, not just the issuer. A list declared alongside a
	// stray AUTH_OIDC_CLIENT_SECRET is a deployment that believes it configured
	// something it did not.
	t.Run("mixing the two forms is refused", func(t *testing.T) {
		for name, single := range map[string]OIDCProvider{
			"an issuer":        {Issuer: "https://idp.test"},
			"a client id":      {ClientID: "0oa..."},
			"a client secret":  {ClientSecret: "shhh"},
			"a claim override": {EmailClaim: "upn"},
		} {
			t.Run(name, func(t *testing.T) {
				cfg := OIDCAuth{
					Providers: []OIDCProvider{{ID: "okta", Issuer: "https://okta.test"}},
					Provider:  single,
				}
				_, err := cfg.Resolve()
				assert.ErrorIs(t, err, ErrOIDCProviderFormsMixed)
			})
		}
	})
}

// The mail defaults changed shape deliberately, so they are pinned here rather
// than against the snapshot.
func TestMailDefaults(t *testing.T) {
	mail := Defaults().Mail

	// No channel, and therefore no destination that quietly swallows mail: a
	// deployment offering password logins has to say where its mail goes.
	assert.Empty(t, mail.Channel)
	assert.False(t, mail.Configured())

	assert.Equal(t, "Lunogram", mail.ProductName)
	assert.Equal(t, "Lunogram", mail.From.Name)
	assert.Equal(t, 30*time.Second, mail.Timeout)
	assert.Equal(t, 587, mail.SMTP.Port)
	assert.Equal(t, mailer.TLSStartTLS, mail.SMTP.TLS)
	assert.Equal(t, "POST", mail.Webhook.Method)
}
