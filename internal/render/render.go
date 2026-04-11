package render

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lunogram/platform/internal/rules"
	"github.com/osteele/liquid"
)

// TimeOption configures the behaviour of RenderTime.
type TimeOption func(*timeOptions)

type timeOptions struct {
	formats  []string
	fallback *time.Location
}

// WithFormats overrides the set of time layouts that RenderTime will attempt,
// in order. The first format that parses successfully wins.
// When not provided, RenderTime defaults to time.RFC3339 only.
func WithFormats(formats ...string) TimeOption {
	return func(o *timeOptions) {
		o.formats = formats
	}
}

// WithFallbackLocation sets the *time.Location applied to parsed times whose
// format does not carry timezone information (e.g. "2006-01-02").
// When not provided, such times are returned as-is (typically UTC).
func WithFallbackLocation(loc *time.Location) TimeOption {
	return func(o *timeOptions) {
		o.fallback = loc
	}
}

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

// RenderTime renders a Liquid template string and parses the result as a time.Time.
// It returns nil when the input pointer is nil or points to an empty string.
//
// By default only time.RFC3339 is attempted. Use WithFormats to supply
// additional layouts and WithFallbackLocation to assign a timezone to
// formats that don't carry one.
//
// Existing call sites that pass no options retain the original behaviour.
func RenderTime(template *string, data map[string]any, opts ...TimeOption) (*time.Time, error) {
	if template == nil || *template == "" {
		return nil, nil
	}

	rendered, err := RenderString(*template, data)
	if err != nil {
		return nil, fmt.Errorf("failed to render template: %w", err)
	}

	rendered = strings.TrimSpace(rendered)
	if rendered == "" {
		return nil, nil
	}

	o := timeOptions{
		formats: []string{time.RFC3339},
	}
	for _, fn := range opts {
		fn(&o)
	}

	var lastErr error
	for _, layout := range o.formats {
		t, err := time.Parse(layout, rendered)
		if err != nil {
			lastErr = err
			continue
		}

		// When the format lacks timezone info and a fallback location was
		// provided, reconstruct the time in that location.
		if o.fallback != nil && !hasTimezone(layout) {
			t = time.Date(t.Year(), t.Month(), t.Day(),
				t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), o.fallback)
		}

		return &t, nil
	}

	return nil, fmt.Errorf("failed to parse %q with any of the configured formats: %w", rendered, lastErr)
}

// hasTimezone reports whether a Go time layout contains an explicit timezone
// component. It looks for the reference-time tokens that encode zone info.
func hasTimezone(layout string) bool {
	// Reference-time timezone tokens: "MST", "Z07", "Z07:00", "Z0700",
	// "-07", "-07:00", "-0700", "+07", etc.
	for _, tok := range []string{"MST", "Z07", "Z0700", "Z07:00", "-07", "-0700", "-07:00"} {
		if strings.Contains(layout, tok) {
			return true
		}
	}
	return false
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
