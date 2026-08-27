package webhook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testEnvelope = `{
  "event": "project.created",
  "version": "v1",
  "occurred_at": "2026-01-02T03:04:05Z",
  "actor": {"type": "admin", "id": "admin-1", "organization_id": "org-1", "project_id": ""},
  "payload": {"project": {"id": "p1", "name": "Acme"}}
}`

func TestParseTemplateInline(t *testing.T) {
	t.Parallel()

	template, err := ParseTemplate("t", `function(ctx) { name: ctx.payload.project.name, by: ctx.actor.id }`, "")
	require.NoError(t, err)

	out, err := template.Render([]byte(testEnvelope))
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	assert.Equal(t, "Acme", got["name"])
	assert.Equal(t, "admin-1", got["by"])
}

func TestParseTemplateFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "hook.jsonnet")
	require.NoError(t, os.WriteFile(path, []byte(`function(ctx) { e: ctx.event }`), 0o600))

	t.Run("absolute", func(t *testing.T) {
		template, err := ParseTemplate("t", "file://"+path, "")
		require.NoError(t, err)
		out, err := template.Render([]byte(testEnvelope))
		require.NoError(t, err)
		assert.JSONEq(t, `{"e":"project.created"}`, string(out))
	})

	t.Run("relative to the config directory", func(t *testing.T) {
		template, err := ParseTemplate("t", "file://hook.jsonnet", dir)
		require.NoError(t, err)
		out, err := template.Render([]byte(testEnvelope))
		require.NoError(t, err)
		assert.JSONEq(t, `{"e":"project.created"}`, string(out))
	})

	t.Run("missing file fails at parse time", func(t *testing.T) {
		_, err := ParseTemplate("t", "file://"+filepath.Join(dir, "nope.jsonnet"), "")
		require.Error(t, err)
	})
}

func TestParseTemplateRejectsBadInput(t *testing.T) {
	t.Parallel()

	for name, ref := range map[string]string{
		"syntax error":  `function(ctx) { name: ctx.payload.project.name`,
		"empty":         "   ",
		"empty file://": "file://",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseTemplate("t", ref, "")
			assert.Error(t, err)
		})
	}
}

func TestTemplateRenderErrorsOnMissingField(t *testing.T) {
	t.Parallel()

	template, err := ParseTemplate("t", `function(ctx) { x: ctx.payload.nope.deeper }`, "")
	require.NoError(t, err, "a reference to a missing field is valid jsonnet, so it parses")

	_, err = template.Render([]byte(testEnvelope))
	require.Error(t, err, "and fails at render time")
}

func TestTemplateImportsAreDisabled(t *testing.T) {
	t.Parallel()

	template, err := ParseTemplate("t", `function(ctx) { leaked: importstr "/etc/passwd" }`, "")
	require.NoError(t, err)

	_, err = template.Render([]byte(testEnvelope))
	require.Error(t, err, "templates must not be able to read the filesystem")
}

// TestDefaultProjectCreatedTemplate locks the embedded template to the wire
// shape the previous hardcoded implementation produced, so upgrading the engine
// is not a breaking change for an existing receiver.
func TestDefaultProjectCreatedTemplate(t *testing.T) {
	t.Parallel()

	template, err := defaultTemplate("project.created")
	require.NoError(t, err)

	out, err := template.Render([]byte(testEnvelope))
	require.NoError(t, err)

	assert.JSONEq(t, `{
	  "event": "project.created",
	  "timestamp": "2026-01-02T03:04:05Z",
	  "project": {"id": "p1", "name": "Acme"}
	}`, string(out))
}

func TestDefaultTemplateMissing(t *testing.T) {
	t.Parallel()

	_, err := defaultTemplate("nope.event")
	require.Error(t, err)
}
