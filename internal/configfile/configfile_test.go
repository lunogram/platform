package configfile

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInterpolateExpandsReferences(t *testing.T) {
	t.Setenv("TOKEN", "s3cr3t")

	out, err := Interpolate([]byte("auth: ${TOKEN}\n"))
	require.NoError(t, err)
	assert.Equal(t, "auth: s3cr3t\n", string(out))
}

func TestInterpolateRejectsUnsetReferences(t *testing.T) {
	_, err := Interpolate([]byte("a: ${MISSING_ONE}\nb: ${MISSING_TWO}\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MISSING_ONE")
	assert.Contains(t, err.Error(), "MISSING_TWO")
}

// A newline in an interpolated value does not land inside the scalar it was
// written into: it ends the line, and everything after it is parsed as though
// the operator had written it into the document.
func TestInterpolateRejectsLineBreaks(t *testing.T) {
	t.Setenv("HTML", "<p>hello</p>\n<p>world</p>")

	_, err := Interpolate([]byte("html: ${HTML}\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTML")
	assert.Contains(t, err.Error(), "line break")
	assert.Contains(t, err.Error(), Base64Scheme)
}

// A '#' stays legal, because it is legitimate inside a password and rejecting
// it would break deployments that already carry one.
func TestInterpolateAllowsHash(t *testing.T) {
	t.Setenv("PASSWORD", "a#b")

	out, err := Interpolate([]byte(`password: "${PASSWORD}"` + "\n"))
	require.NoError(t, err)
	assert.Contains(t, string(out), "a#b")
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
