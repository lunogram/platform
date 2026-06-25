// Package params provides helpers for parsing HTTP query and path parameters
// that come from OpenAPI-generated handlers.
package params

import "strings"

// Split splits a comma-separated parameter value into its trimmed, non-empty
// elements. It returns nil when the input is empty so that callers can safely
// compose it with helpers like ptr.From for optional query parameters.
func Split(value string) []string {
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
