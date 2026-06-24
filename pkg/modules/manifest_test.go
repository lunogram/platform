package modules

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONSchemaPropertyHidden(t *testing.T) {
	t.Parallel()

	type test struct {
		property JSONSchemaProperty
		wantKey  bool
	}

	tests := map[string]test{
		"hidden true is serialized": {
			property: JSONSchemaProperty{
				Name:   "secret_key",
				Schema: &JSONSchema{Type: "string"},
				Hidden: true,
			},
			wantKey: true,
		},
		"hidden false is omitted": {
			property: JSONSchemaProperty{
				Name:   "api_key",
				Schema: &JSONSchema{Type: "string"},
				Hidden: false,
			},
			wantKey: false,
		},
		"hidden zero-value is omitted": {
			property: JSONSchemaProperty{
				Name:   "api_key",
				Schema: &JSONSchema{Type: "string"},
			},
			wantKey: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(test.property)
			require.NoError(t, err)

			var raw map[string]any
			err = json.Unmarshal(data, &raw)
			require.NoError(t, err)

			_, hasHidden := raw["hidden"]
			assert.Equal(t, test.wantKey, hasHidden)

			if test.wantKey {
				assert.Equal(t, true, raw["hidden"])
			}
		})
	}
}

func TestJSONSchemaPropertyHiddenDeserialization(t *testing.T) {
	t.Parallel()

	type test struct {
		input      string
		wantHidden bool
	}

	tests := map[string]test{
		"with hidden true": {
			input:      `{"name":"secret","schema":{"type":"string"},"hidden":true}`,
			wantHidden: true,
		},
		"with hidden false": {
			input:      `{"name":"api_key","schema":{"type":"string"},"hidden":false}`,
			wantHidden: false,
		},
		"without hidden field": {
			input:      `{"name":"api_key","schema":{"type":"string"}}`,
			wantHidden: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var prop JSONSchemaProperty
			err := json.Unmarshal([]byte(test.input), &prop)
			require.NoError(t, err)
			assert.Equal(t, test.wantHidden, prop.Hidden)
		})
	}
}
