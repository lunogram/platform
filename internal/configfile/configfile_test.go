package configfile

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type target struct {
	Auth     string            `yaml:"auth"`
	Port     int               `yaml:"port"`
	Timeout  time.Duration     `yaml:"timeout"`
	Enabled  bool              `yaml:"enabled"`
	HTML     string            `yaml:"html"`
	Settings map[string]string `yaml:"settings"`
	List     []string          `yaml:"list"`
}

func TestDecodeExpandsReferences(t *testing.T) {
	t.Setenv("TOKEN", "s3cr3t")

	var out target
	require.NoError(t, Decode([]byte("auth: ${TOKEN}\n"), &out))
	assert.Equal(t, "s3cr3t", out.Auth)
}

func TestDecodeRejectsUnsetReferences(t *testing.T) {
	err := Decode([]byte("auth: ${MISSING_ONE}\nhtml: ${MISSING_TWO}\n"), &target{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MISSING_ONE")
	assert.Contains(t, err.Error(), "MISSING_TWO")
}

// The point of expanding the parsed document rather than the bytes: a value is
// data by the time it exists, so a secret is free to contain whatever a secret
// contains. Every one of these used to either corrupt the document or be
// silently truncated.
func TestDecodeCarriesValuesTheTextFormCouldNot(t *testing.T) {
	tests := map[string]string{
		"line breaks":     "<p>one</p>\n<p>two</p>",
		"a hash":          "pa#ssword",
		"a trailing note": "secret # not a comment",
		"a colon":         "key: value",
		"a leading dash":  "- not an item",
		"flow syntax":     "{a: b}",
		"an alias":        "*anchor",
		"an anchor":       "&anchor evil",
		"a merge key":     "<<",
		"a fake mapping":  "x\nsettings:\n  injected: yes",
		"a tag":           "!!binary aGk=",
		"quotes":          `she said "hello" and 'goodbye'`,
	}

	for name, value := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv("SECRET", value)

			var out target
			require.NoError(t, Decode([]byte("auth: \"${SECRET}\"\nsettings:\n  a: b\n"), &out))

			assert.Equal(t, value, out.Auth, "the value must arrive exactly as it was set")
			assert.Equal(t, map[string]string{"a": "b"}, out.Settings,
				"nothing in the value may reach the rest of the document")
		})
	}
}

// A plain scalar was written without quotes, so its type comes from what it
// says. Quoting it asks for a string.
func TestDecodeResolvesPlainScalarTypes(t *testing.T) {
	t.Setenv("PORT", "2525")
	t.Setenv("TIMEOUT", "45s")
	t.Setenv("ENABLED", "true")

	var out target
	require.NoError(t, Decode([]byte("port: ${PORT}\ntimeout: ${TIMEOUT}\nenabled: ${ENABLED}\n"), &out))

	assert.Equal(t, 2525, out.Port)
	assert.Equal(t, 45*time.Second, out.Timeout)
	assert.True(t, out.Enabled)
}

func TestDecodeKeepsQuotedValuesAsStrings(t *testing.T) {
	t.Setenv("VALUE", "0755")

	var out target
	require.NoError(t, Decode([]byte(`auth: "${VALUE}"`+"\n"), &out))
	assert.Equal(t, "0755", out.Auth, "quoting asked for a string, not an octal number")
}

// A key assembled from the environment could rename or shadow a setting, which
// is a sharper thing than supplying a value.
func TestDecodeRefusesKeysFromTheEnvironment(t *testing.T) {
	t.Setenv("KEY", "auth")

	err := Decode([]byte("settings:\n  ${KEY}: value\n"), &target{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "keys cannot be built from the environment")
}

// A variable holding a reference yields that text rather than reaching for
// another variable.
func TestDecodeDoesNotExpandRecursively(t *testing.T) {
	t.Setenv("OUTER", "${INNER}")
	t.Setenv("INNER", "should not be reached")

	var out target
	require.NoError(t, Decode([]byte(`auth: "${OUTER}"`+"\n"), &out))
	assert.Equal(t, "${INNER}", out.Auth)
}

// An inline JSONNet body or a shell snippet in the file needs to write the
// literal text.
func TestDecodeEscapesDoubledReferences(t *testing.T) {
	var out target
	require.NoError(t, Decode([]byte(`auth: "$${NOT_A_VARIABLE}"`+"\n"), &out))
	assert.Equal(t, "${NOT_A_VARIABLE}", out.Auth)
}

// Expansion runs on the parsed document, and a comment is not part of it.
func TestDecodeIgnoresComments(t *testing.T) {
	var out target
	require.NoError(t, Decode([]byte("# see ${UNDEFINED_VARIABLE} for details\nauth: fixed\n"), &out))
	assert.Equal(t, "fixed", out.Auth)
}

func TestDecodeExpandsInsideSequences(t *testing.T) {
	t.Setenv("DRIVER", "password")

	var out target
	require.NoError(t, Decode([]byte("list:\n  - ${DRIVER}\n  - clerk\n"), &out))
	assert.Equal(t, []string{"password", "clerk"}, out.List)
}

func TestDecodeRejectsUnknownKeys(t *testing.T) {
	err := Decode([]byte("nonsense: 1\n"), &target{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonsense")
}

// An empty file is "configure nothing", not "unset everything".
func TestDecodeLeavesTheTargetAloneWhenEmpty(t *testing.T) {
	out := target{Auth: "already set"}
	require.NoError(t, Decode(nil, &out))
	assert.Equal(t, "already set", out.Auth)
}

func TestDecodeReportsMalformedYAML(t *testing.T) {
	require.Error(t, Decode([]byte("auth: [unterminated\n"), &target{}))
}

func TestResolveBase64(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("<h1>hello</h1>"))

	out, err := Resolve("verify_email", Base64Scheme+encoded, "")
	require.NoError(t, err)
	assert.Equal(t, "<h1>hello</h1>", string(out))
}

// GNU base64 wraps at 76 columns by default, so a pasted payload arrives with
// line breaks in it.
func TestResolveBase64IgnoresWrapping(t *testing.T) {
	raw := strings.Repeat("<p>a paragraph of html</p>", 20)
	encoded := base64.StdEncoding.EncodeToString([]byte(raw))

	var wrapped strings.Builder
	for i := 0; i < len(encoded); i += 76 {
		wrapped.WriteString(encoded[i:min(i+76, len(encoded))])
		wrapped.WriteString("\n")
	}

	out, err := Resolve("verify_email", Base64Scheme+wrapped.String(), "")
	require.NoError(t, err)
	assert.Equal(t, raw, string(out))
}

func TestResolveBase64Unpadded(t *testing.T) {
	encoded := base64.RawStdEncoding.EncodeToString([]byte("unpadded"))

	out, err := Resolve("verify_email", Base64Scheme+encoded, "")
	require.NoError(t, err)
	assert.Equal(t, "unpadded", string(out))
}

func TestResolveBase64Invalid(t *testing.T) {
	_, err := Resolve("verify_email", Base64Scheme+"not!base64!", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "verify_email")
	assert.Contains(t, err.Error(), "not valid base64")
}

func TestResolveFileRelativeToBaseDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "body.tmpl"), []byte("hello"), 0o600))

	out, err := Resolve("verify_email", FileScheme+"body.tmpl", dir)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(out))
}

func TestResolveFileMissing(t *testing.T) {
	_, err := Resolve("verify_email", FileScheme+"absent.tmpl", t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "verify_email")
}

func TestResolveLiteral(t *testing.T) {
	out, err := Resolve("body", "function(ctx) { event: ctx.event }", "")
	require.NoError(t, err)
	assert.Equal(t, "function(ctx) { event: ctx.event }", string(out))
}

// A complex key is still a key all the way down, whether it is a sequence or a
// mapping: everything inside it contributes to the key's identity.
func TestDecodeRefusesReferencesInsideComplexKeys(t *testing.T) {
	t.Setenv("KEY", "auth")

	documents := map[string]string{
		"sequence": "settings:\n  ? [\"${KEY}\"]\n  : value\n",
		"mapping":  "settings:\n  ? {a: \"${KEY}\"}\n  : value\n",
	}

	for name, document := range documents {
		t.Run(name, func(t *testing.T) {
			err := Decode([]byte(document), &target{})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "keys cannot be built from the environment")
		})
	}
}
