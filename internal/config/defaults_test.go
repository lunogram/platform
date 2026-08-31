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
// The mail section is excluded because it is deliberately redesigned in this
// change -- a channel replaced a bare host, and the sender moved out of the
// transport -- and is asserted separately below. The captured auth.JWKS was
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

	assert.Equal(t, former, current)
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
