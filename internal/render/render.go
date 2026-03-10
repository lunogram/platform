package render

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lunogram/platform/internal/rules"
	"github.com/osteele/liquid"
)

const TemplatePrefix = "{{"

// RenderString renders a Liquid template string against the provided data bindings.
func RenderString(template string, data map[string]any) (string, error) {
	if !strings.Contains(template, TemplatePrefix) {
		return template, nil
	}

	engine := liquid.NewEngine()
	rendered, err := engine.ParseAndRenderString(template, data)
	if err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	return rendered, nil
}

// RenderJSON walks a JSON value and renders all Liquid template strings
// found within it. It handles objects, arrays, and string values recursively.
// Non-string values (numbers, booleans, null) are left unchanged.
func RenderJSON(raw json.RawMessage, data map[string]any) (json.RawMessage, error) {
	if raw == nil {
		return nil, nil
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	resolved, err := renderValue(value, data)
	if err != nil {
		return nil, err
	}

	result, err := json.Marshal(resolved)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resolved JSON: %w", err)
	}

	return result, nil
}

// RenderRuleSet returns a copy of the RuleSet with all Liquid template strings
// in leaf rule Value fields resolved against the provided data. Wrapper rules
// (whose Value is an event name selected from a Combobox) are left unchanged.
// The original RuleSet is never mutated.
func RenderRuleSet(rs rules.RuleSet, data map[string]any) (rules.RuleSet, error) {
	rendered, err := renderRule(rs.Rule, data)
	if err != nil {
		return rs, fmt.Errorf("failed to render rule set: %w", err)
	}
	return rules.RuleSet{Rule: rendered}, nil
}

// renderRule recursively copies a Rule tree, resolving Liquid templates in
// leaf (non-wrapper) string Value fields.
func renderRule(r rules.Rule, data map[string]any) (rules.Rule, error) {
	out := r

	// Resolve Value on leaf rules only (wrapper Value is an event name, not templated)
	if !r.IsWrapper() {
		if s, ok := r.Value.(string); ok {
			rendered, err := RenderString(s, data)
			if err != nil {
				return out, fmt.Errorf("rule %s value: %w", r.UUID, err)
			}
			out.Value = rendered
		}
	}

	// Deep-copy and resolve children
	if len(r.Children) > 0 {
		out.Children = make([]rules.Rule, len(r.Children))
		for i, child := range r.Children {
			rendered, err := renderRule(child, data)
			if err != nil {
				return out, err
			}
			out.Children[i] = rendered
		}
	}

	// Resolve UserMatch.MemberConditions if present
	if r.UserMatch != nil {
		um := *r.UserMatch
		if r.UserMatch.MemberConditions != nil {
			mc, err := renderRule(*r.UserMatch.MemberConditions, data)
			if err != nil {
				return out, fmt.Errorf("user match conditions: %w", err)
			}
			um.MemberConditions = &mc
		}
		out.UserMatch = &um
	}

	return out, nil
}

func renderValue(value any, data map[string]any) (any, error) {
	switch v := value.(type) {
	case string:
		rendered, err := RenderString(v, data)
		if err != nil {
			return nil, err
		}
		return rendered, nil
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, val := range v {
			resolved, err := renderValue(val, data)
			if err != nil {
				return nil, fmt.Errorf("field %q: %w", key, err)
			}
			result[key] = resolved
		}
		return result, nil
	case []any:
		result := make([]any, len(v))
		for i, val := range v {
			resolved, err := renderValue(val, data)
			if err != nil {
				return nil, fmt.Errorf("index %d: %w", i, err)
			}
			result[i] = resolved
		}
		return result, nil
	default:
		// numbers, booleans, null — leave unchanged
		return value, nil
	}
}
