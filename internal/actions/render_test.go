package actions

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderRawMessage(t *testing.T) {
	t.Parallel()

	type test struct {
		raw       json.RawMessage
		variables map[string]any
		want      string
		wantErr   bool
	}

	tests := map[string]test{
		"nil variables returns raw unchanged": {
			raw:       json.RawMessage(`{"url":"https://example.com"}`),
			variables: nil,
			want:      `{"url":"https://example.com"}`,
		},
		"empty variables returns raw unchanged": {
			raw:       json.RawMessage(`{"url":"https://example.com"}`),
			variables: map[string]any{},
			want:      `{"url":"https://example.com"}`,
		},
		"nil raw returns nil": {
			raw:       nil,
			variables: map[string]any{"name": "test"},
			want:      "",
		},
		"empty raw returns empty": {
			raw:       json.RawMessage(``),
			variables: map[string]any{"name": "test"},
			want:      "",
		},
		"simple string substitution": {
			raw:       json.RawMessage(`{"url":"https://api.com/{{.user_id}}"}`),
			variables: map[string]any{"user_id": "abc123"},
			want:      `{"url":"https://api.com/abc123"}`,
		},
		"multiple variables in same string": {
			raw:       json.RawMessage(`{"url":"https://{{.host}}/{{.path}}"}`),
			variables: map[string]any{"host": "api.com", "path": "v1/users"},
			want:      `{"url":"https://api.com/v1/users"}`,
		},
		"nested object substitution": {
			raw: json.RawMessage(`{"headers":{"Authorization":"Bearer {{.token}}","X-Custom":"{{.custom_header}}"}}`),
			variables: map[string]any{
				"token":         "my-secret",
				"custom_header": "value",
			},
			want: `{"headers":{"Authorization":"Bearer my-secret","X-Custom":"value"}}`,
		},
		"integer variable substitution": {
			raw:       json.RawMessage(`{"body":"count is {{.count}}"}`),
			variables: map[string]any{"count": 42},
			want:      `{"body":"count is 42"}`,
		},
		"boolean variable substitution": {
			raw:       json.RawMessage(`{"body":"active is {{.active}}"}`),
			variables: map[string]any{"active": true},
			want:      `{"body":"active is true"}`,
		},
		"no placeholders leaves raw unchanged": {
			raw:       json.RawMessage(`{"url":"https://example.com/static"}`),
			variables: map[string]any{"name": "test"},
			want:      `{"url":"https://example.com/static"}`,
		},
		"missing variable renders as empty string": {
			raw:       json.RawMessage(`{"url":"https://api.com/{{.missing}}"}`),
			variables: map[string]any{"name": "test"},
			want:      `{"url":"https://api.com/"}`,
		},
		"deeply nested structure": {
			raw:       json.RawMessage(`{"level1":{"level2":{"level3":"{{.deep_value}}"}}}`),
			variables: map[string]any{"deep_value": "found"},
			want:      `{"level1":{"level2":{"level3":"found"}}}`,
		},
		"array values are rendered": {
			raw:       json.RawMessage(`{"tags":["{{.env}}-primary","{{.env}}-secondary"]}`),
			variables: map[string]any{"env": "prod"},
			want:      `{"tags":["prod-primary","prod-secondary"]}`,
		},
		"special characters in value are JSON-escaped": {
			raw:       json.RawMessage(`{"body":"{{.msg}}"}`),
			variables: map[string]any{"msg": `hello "world" \n`},
			want:      `{"body":"hello \"world\" \\n"}`,
		},
		"value with newlines is escaped": {
			raw:       json.RawMessage(`{"body":"{{.text}}"}`),
			variables: map[string]any{"text": "line1\nline2"},
			want:      `{"body":"line1\nline2"}`,
		},
		"float variable substitution": {
			raw:       json.RawMessage(`{"price":"{{.p}}"}`),
			variables: map[string]any{"p": 9.99},
			want:      `{"price":"9.99"}`,
		},
		"multiple occurrences of same variable": {
			raw:       json.RawMessage(`{"a":"{{.x}}","b":"{{.x}}"}`),
			variables: map[string]any{"x": "val"},
			want:      `{"a":"val","b":"val"}`,
		},
		"non-template braces with hyphen are preserved": {
			raw:       json.RawMessage(`{"body":"{{some-value}}"}`),
			variables: map[string]any{"name": "test"},
			want:      `{"body":"{{some-value}}"}`,
		},
		"non-template braces with UUID are preserved": {
			raw:       json.RawMessage(`{"id":"{{185c5099-d7b5-42a1-96f5-f1fb175698ba}}"}`),
			variables: map[string]any{"name": "test"},
			want:      `{"id":"{{185c5099-d7b5-42a1-96f5-f1fb175698ba}}"}`,
		},
		"mix of template vars and non-template braces": {
			raw:       json.RawMessage(`{"url":"https://{{.host}}/api","ref":"{{my-ref}}"}`),
			variables: map[string]any{"host": "example.com"},
			want:      `{"url":"https://example.com/api","ref":"{{my-ref}}"}`,
		},
		"handlebars-style expressions are preserved": {
			raw:       json.RawMessage(`{"template":"{{#each items}}{{name}}{{/each}}"}`),
			variables: map[string]any{"name": "test"},
			want:      `{"template":"{{#each items}}{{name}}{{/each}}"}`,
		},
		"non-template braces without dot prefix are preserved": {
			raw:       json.RawMessage(`{"body":"{{username}}"}`),
			variables: map[string]any{"username": "test"},
			want:      `{"body":"{{username}}"}`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := RenderRawMessage(tc.raw, tc.variables)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(got))
		})
	}
}

func TestRenderRawMessage_ValidJSON(t *testing.T) {
	t.Parallel()

	raw := json.RawMessage(`{
		"url": "https://{{.host}}/api/{{.version}}",
		"headers": {
			"Authorization": "Bearer {{.token}}",
			"Content-Type": "application/json"
		},
		"retries": 3,
		"enabled": true
	}`)

	variables := map[string]any{
		"host":    "example.com",
		"version": "v2",
		"token":   "secret-key",
	}

	got, err := RenderRawMessage(raw, variables)
	require.NoError(t, err)

	// The result should be valid JSON.
	var parsed map[string]any
	err = json.Unmarshal(got, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "https://example.com/api/v2", parsed["url"])

	headers, ok := parsed["headers"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Bearer secret-key", headers["Authorization"])
	assert.Equal(t, "application/json", headers["Content-Type"])
	assert.Equal(t, float64(3), parsed["retries"])
	assert.Equal(t, true, parsed["enabled"])
}

func TestJsonEscapeValue(t *testing.T) {
	t.Parallel()

	type test struct {
		input any
		want  string
	}

	tests := map[string]test{
		"plain string": {
			input: "hello",
			want:  "hello",
		},
		"string with double quotes": {
			input: `say "hi"`,
			want:  `say \"hi\"`,
		},
		"string with backslash": {
			input: `path\to\file`,
			want:  `path\\to\\file`,
		},
		"string with newline": {
			input: "line1\nline2",
			want:  `line1\nline2`,
		},
		"string with tab": {
			input: "col1\tcol2",
			want:  `col1\tcol2`,
		},
		"integer": {
			input: 42,
			want:  "42",
		},
		"float": {
			input: 3.14,
			want:  "3.14",
		},
		"boolean true": {
			input: true,
			want:  "true",
		},
		"boolean false": {
			input: false,
			want:  "false",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := jsonEscapeValue(tc.input)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestMarshalAndRender(t *testing.T) {
	t.Parallel()

	t.Run("config map with variables", func(t *testing.T) {
		t.Parallel()

		config := map[string]any{
			"url":    "https://{{.host}}/api/{{.version}}",
			"apiKey": "{{.token}}",
		}
		variables := map[string]any{
			"host":    "example.com",
			"version": "v2",
			"token":   "secret-key",
		}

		got, err := MarshalAndRender(config, variables)
		require.NoError(t, err)

		var parsed map[string]any
		err = json.Unmarshal(got, &parsed)
		require.NoError(t, err)

		assert.Equal(t, "https://example.com/api/v2", parsed["url"])
		assert.Equal(t, "secret-key", parsed["apiKey"])
	})

	t.Run("nil value returns null JSON", func(t *testing.T) {
		t.Parallel()

		got, err := MarshalAndRender(nil, map[string]any{"name": "test"})
		require.NoError(t, err)
		assert.Equal(t, "null", string(got))
	})

	t.Run("nil variables skips rendering", func(t *testing.T) {
		t.Parallel()

		config := map[string]any{"key": "value"}
		got, err := MarshalAndRender(config, nil)
		require.NoError(t, err)

		var parsed map[string]any
		err = json.Unmarshal(got, &parsed)
		require.NoError(t, err)
		assert.Equal(t, "value", parsed["key"])
	})

	t.Run("payload with nested structure", func(t *testing.T) {
		t.Parallel()

		payload := map[string]any{
			"method":   "POST",
			"endpoint": "https://{{.host}}/webhook",
			"headers": map[string]any{
				"Authorization": "Bearer {{.token}}",
			},
		}
		variables := map[string]any{
			"host":  "api.example.com",
			"token": "my-secret",
		}

		got, err := MarshalAndRender(payload, variables)
		require.NoError(t, err)

		var parsed map[string]any
		err = json.Unmarshal(got, &parsed)
		require.NoError(t, err)

		assert.Equal(t, "POST", parsed["method"])
		assert.Equal(t, "https://api.example.com/webhook", parsed["endpoint"])

		headers, ok := parsed["headers"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "Bearer my-secret", headers["Authorization"])
	})

	t.Run("special characters in variable are JSON-safe", func(t *testing.T) {
		t.Parallel()

		config := map[string]any{
			"body": "{{.msg}}",
		}
		variables := map[string]any{
			"msg": "hello \"world\"\nnewline",
		}

		got, err := MarshalAndRender(config, variables)
		require.NoError(t, err)

		var parsed map[string]any
		err = json.Unmarshal(got, &parsed)
		require.NoError(t, err)
		assert.Equal(t, "hello \"world\"\nnewline", parsed["body"])
	})
}
