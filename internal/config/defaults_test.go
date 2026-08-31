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

// A deployment that names the oidc driver and nothing else gets the claims and
// scopes every provider documents, so the common case is three variables.
func TestOIDCDefaults(t *testing.T) {
	oidc := Defaults().Auth.OIDC

	assert.False(t, oidc.Configured(), "no issuer, no client: the driver is off unless it is configured")
	assert.Equal(t, []string{"openid", "email", "profile"}, oidc.Scopes)
	assert.Equal(t, "email", oidc.EmailClaim)
	assert.Equal(t, "given_name", oidc.GivenNameClaim)
	assert.Equal(t, "family_name", oidc.FamilyNameClaim)
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
