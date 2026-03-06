package actions

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// varPattern matches our template variable syntax: {{.identifier}} with
// optional whitespace, e.g. {{ .foo }}, {{.bar}}. Identifiers must start
// with a letter or underscore and contain only word characters.
var varPattern = regexp.MustCompile(`\{\{\s*\.(\w+)\s*\}\}`)

// MarshalAndRender marshals the given value to JSON and then performs variable
// substitution on the resulting bytes. Template placeholders use the
// {{.variable_name}} syntax. Unmatched placeholders are left empty.
func MarshalAndRender(v any, variables map[string]any) (json.RawMessage, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal value: %w", err)
	}

	return RenderRawMessage(raw, variables)
}

// RenderRawMessage performs variable substitution on raw JSON bytes.
// Template placeholders use the {{.variable_name}} syntax. Unmatched
// placeholders are replaced with an empty string.
//
// Only {{.identifier}} patterns are recognised as template variables.
// Any other {{…}} sequences (e.g. Handlebars, UUIDs, hyphenated names)
// are left untouched.
//
// This operates directly on the serialized JSON, avoiding the need to
// unmarshal into intermediate Go types, render, and re-marshal.
func RenderRawMessage(raw json.RawMessage, variables map[string]any) (json.RawMessage, error) {
	if len(variables) == 0 || len(raw) == 0 {
		return raw, nil
	}

	// Build a lookup map where every value is JSON-escaped so that
	// substitution into a JSON document preserves validity.
	data := make(map[string]string, len(variables))
	for k, v := range variables {
		data[k] = jsonEscapeValue(v)
	}

	result := varPattern.ReplaceAllStringFunc(string(raw), func(match string) string {
		// Extract the variable name from the submatch.
		subs := varPattern.FindStringSubmatch(match)
		if len(subs) < 2 {
			return match
		}
		name := subs[1]

		if val, ok := data[name]; ok {
			return val
		}
		// Missing variable → empty string.
		return ""
	})

	return json.RawMessage(result), nil
}

// jsonEscapeValue converts a variable value into a string that is safe to
// embed directly inside a JSON document. For string values the surrounding
// quotes are stripped so the result can be placed inside an existing JSON
// string value. Non-string values (numbers, booleans, etc.) are formatted
// with [fmt.Sprintf].
func jsonEscapeValue(v any) string {
	switch val := v.(type) {
	case string:
		b, _ := json.Marshal(val)
		// Strip the surrounding quotes produced by json.Marshal.
		return string(b[1 : len(b)-1])
	default:
		return fmt.Sprintf("%v", val)
	}
}
