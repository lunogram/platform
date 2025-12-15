package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePaths(t *testing.T) {
	type test struct {
		input    map[string]any
		expected Paths
	}

	tests := map[string]test{
		"empty map": {
			input:    map[string]any{},
			expected: Paths(nil),
		},
		"simple string": {
			input: map[string]any{
				"name": "John",
			},
			expected: Paths{
				{Path: ".name", Type: TypeString},
			},
		},
		"multiple primitives": {
			input: map[string]any{
				"name":   "John",
				"age":    float64(30),
				"active": true,
			},
			expected: Paths{
				{Path: ".name", Type: TypeString},
				{Path: ".age", Type: TypeNumber},
				{Path: ".active", Type: TypeBool},
			},
		},
		"nested object": {
			input: map[string]any{
				"user": map[string]any{
					"name": "John",
					"age":  float64(30),
				},
			},
			expected: Paths{
				{Path: ".user", Type: TypeObject},
				{Path: ".user.name", Type: TypeString},
				{Path: ".user.age", Type: TypeNumber},
			},
		},
		"array of primitives": {
			input: map[string]any{
				"tags": []any{"go", "testing", "json"},
			},
			expected: Paths{
				{Path: ".tags", Type: TypeArray},
				{Path: ".tags[]", Type: TypeString},
			},
		},
		"array of objects": {
			input: map[string]any{
				"users": []any{
					map[string]any{
						"name": "John",
						"age":  float64(30),
					},
					map[string]any{
						"name": "Jane",
						"age":  float64(25),
					},
				},
			},
			expected: Paths{
				{Path: ".users", Type: TypeArray},
				{Path: ".users[]", Type: TypeObject},
				{Path: ".users[].name", Type: TypeString},
				{Path: ".users[].age", Type: TypeNumber},
			},
		},
		"null value": {
			input: map[string]any{
				"middle_name": nil,
			},
			expected: Paths{
				{Path: ".middle_name", Type: TypeNull},
			},
		},
		"complex nested structure": {
			input: map[string]any{
				"user": map[string]any{
					"name": "John",
					"profile": map[string]any{
						"bio": "Developer",
						"settings": map[string]any{
							"notifications": true,
						},
					},
					"tags": []any{"admin", "developer"},
				},
				"count": float64(42),
			},
			expected: Paths{
				{Path: ".user", Type: TypeObject},
				{Path: ".user.name", Type: TypeString},
				{Path: ".user.profile", Type: TypeObject},
				{Path: ".user.profile.bio", Type: TypeString},
				{Path: ".user.profile.settings", Type: TypeObject},
				{Path: ".user.profile.settings.notifications", Type: TypeBool},
				{Path: ".user.tags", Type: TypeArray},
				{Path: ".user.tags[]", Type: TypeString},
				{Path: ".count", Type: TypeNumber},
			},
		},
		"nested arrays": {
			input: map[string]any{
				"matrix": []any{
					[]any{float64(1), float64(2)},
					[]any{float64(3), float64(4)},
				},
			},
			expected: Paths{
				{Path: ".matrix", Type: TypeArray},
				{Path: ".matrix[]", Type: TypeArray},
				{Path: ".matrix[][]", Type: TypeNumber},
			},
		},
		"mixed types in object": {
			input: map[string]any{
				"string": "value",
				"number": float64(123),
				"bool":   true,
				"null":   nil,
				"object": map[string]any{"key": "value"},
				"array":  []any{float64(1), float64(2)},
			},
			expected: Paths{
				{Path: ".string", Type: TypeString},
				{Path: ".number", Type: TypeNumber},
				{Path: ".bool", Type: TypeBool},
				{Path: ".null", Type: TypeNull},
				{Path: ".object", Type: TypeObject},
				{Path: ".object.key", Type: TypeString},
				{Path: ".array", Type: TypeArray},
				{Path: ".array[]", Type: TypeNumber},
			},
		},
		"empty array": {
			input: map[string]any{
				"items": []any{},
			},
			expected: Paths{
				{Path: ".items", Type: TypeArray},
			},
		},
		"empty nested object": {
			input: map[string]any{
				"metadata": map[string]any{},
			},
			expected: Paths{
				{Path: ".metadata", Type: TypeObject},
			},
		},
		"mixed type array": {
			input: map[string]any{
				"mixed": []any{"text", float64(42), true, nil},
			},
			expected: Paths{
				{Path: ".mixed", Type: TypeArray},
				{Path: ".mixed[]", Type: TypeString},
				{Path: ".mixed[]", Type: TypeNumber},
				{Path: ".mixed[]", Type: TypeBool},
				{Path: ".mixed[]", Type: TypeNull},
			},
		},
		"array with mixed objects": {
			input: map[string]any{
				"items": []any{
					map[string]any{"name": "John", "age": float64(30)},
					map[string]any{"name": "Jane", "email": "jane@example.com"},
				},
			},
			expected: Paths{
				{Path: ".items", Type: TypeArray},
				{Path: ".items[]", Type: TypeObject},
				{Path: ".items[].name", Type: TypeString},
				{Path: ".items[].age", Type: TypeNumber},
				{Path: ".items[].email", Type: TypeString},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := ParsePaths(tc.input)
			assert.ElementsMatch(t, tc.expected, result)
		})
	}
}

func TestCollectMapPaths(t *testing.T) {
	type test struct {
		initialPaths Paths
		prefix       string
		input        map[string]any
		expected     Paths
	}

	tests := map[string]test{
		"empty map": {
			initialPaths: nil,
			prefix:       "",
			input:        map[string]any{},
			expected:     Paths(nil),
		},
		"single key": {
			initialPaths: nil,
			prefix:       ".root",
			input: map[string]any{
				"field": "value",
			},
			expected: Paths{
				{Path: ".root.field", Type: TypeString},
			},
		},
		"with existing paths": {
			initialPaths: Paths{
				{Path: ".existing", Type: TypeString},
			},
			prefix: ".root",
			input: map[string]any{
				"field": "value",
			},
			expected: Paths{
				{Path: ".existing", Type: TypeString},
				{Path: ".root.field", Type: TypeString},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := collectMapPaths(tc.initialPaths, make(map[Path]struct{}), tc.prefix, tc.input)
			assert.ElementsMatch(t, tc.expected, result)
		})
	}
}

func TestCollectArrayPaths(t *testing.T) {
	type test struct {
		initialPaths Paths
		prefix       string
		input        []any
		expected     Paths
	}

	tests := map[string]test{
		"empty array": {
			initialPaths: nil,
			prefix:       ".items",
			input:        []any{},
			expected:     Paths(nil),
		},
		"array of strings": {
			initialPaths: nil,
			prefix:       ".tags",
			input:        []any{"first", "second"},
			expected: Paths{
				{Path: ".tags[]", Type: TypeString},
			},
		},
		"array of numbers": {
			initialPaths: nil,
			prefix:       ".numbers",
			input:        []any{float64(1), float64(2), float64(3)},
			expected: Paths{
				{Path: ".numbers[]", Type: TypeNumber},
			},
		},
		"with existing paths": {
			initialPaths: Paths{
				{Path: ".existing", Type: TypeString},
			},
			prefix: ".items",
			input:  []any{"value"},
			expected: Paths{
				{Path: ".existing", Type: TypeString},
				{Path: ".items[]", Type: TypeString},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := collectArrayPaths(tc.initialPaths, make(map[Path]struct{}), tc.prefix, tc.input)
			assert.ElementsMatch(t, tc.expected, result)
		})
	}
}

func TestCollectAny(t *testing.T) {
	type test struct {
		initialPaths Paths
		prefix       string
		input        any
		expected     Paths
	}

	tests := map[string]test{
		"string value": {
			initialPaths: nil,
			prefix:       ".field",
			input:        "text",
			expected: Paths{
				{Path: ".field", Type: TypeString},
			},
		},
		"number value": {
			initialPaths: nil,
			prefix:       ".count",
			input:        float64(42),
			expected: Paths{
				{Path: ".count", Type: TypeNumber},
			},
		},
		"bool value": {
			initialPaths: nil,
			prefix:       ".enabled",
			input:        true,
			expected: Paths{
				{Path: ".enabled", Type: TypeBool},
			},
		},
		"null value": {
			initialPaths: nil,
			prefix:       ".optional",
			input:        nil,
			expected: Paths{
				{Path: ".optional", Type: TypeNull},
			},
		},
		"object value": {
			initialPaths: nil,
			prefix:       ".obj",
			input: map[string]any{
				"key": "value",
			},
			expected: Paths{
				{Path: ".obj", Type: TypeObject},
				{Path: ".obj.key", Type: TypeString},
			},
		},
		"array value": {
			initialPaths: nil,
			prefix:       ".arr",
			input:        []any{float64(1), float64(2)},
			expected: Paths{
				{Path: ".arr", Type: TypeArray},
				{Path: ".arr[]", Type: TypeNumber},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result := collectAny(tc.initialPaths, make(map[Path]struct{}), tc.prefix, tc.input)
			assert.ElementsMatch(t, tc.expected, result)
		})
	}
}
