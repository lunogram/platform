package eval

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lunogram/platform/internal/rules"
)

// Evaluator evaluates rules against in-memory data
type Evaluator struct{}

// NewEvaluator creates a new Evaluator
func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

// Evaluate evaluates a RuleSet against the provided data from journey steps
func (e *Evaluator) Evaluate(ruleSet rules.RuleSet, data map[string]any) (bool, error) {
	return e.evaluateRule(&ruleSet.Rule, data)
}

// evaluateRule recursively evaluates a rule node
func (e *Evaluator) evaluateRule(rule *rules.Rule, data map[string]any) (bool, error) {
	// Frequency-based rules cannot be evaluated in memory
	if rule.Frequency != nil {
		return false, fmt.Errorf("cannot evaluate frequency-based rules in memory")
	}

	// Handle wrapper nodes with logical operators
	if rule.IsWrapper() {
		return e.evaluateWrapper(rule, data)
	}

	// Handle leaf nodes - all rules now use the same data context
	value, err := e.extractValue(data, rule.Path)
	if err != nil {
		return false, err
	}

	return e.evaluateComparison(value, rule.Operator, rule.Value, rule.Type)
}

// evaluateWrapper evaluates wrapper nodes with AND/OR logic
func (e *Evaluator) evaluateWrapper(rule *rules.Rule, data map[string]any) (bool, error) {
	if !rule.HasChildren() {
		return true, nil
	}

	switch rule.Operator {
	case rules.OperatorAnd:
		// All children must evaluate to true
		for i := range rule.Children {
			result, err := e.evaluateRule(&rule.Children[i], data)
			if err != nil {
				return false, err
			}
			if !result {
				return false, nil
			}
		}
		return true, nil

	case rules.OperatorOr:
		// At least one child must evaluate to true
		for i := range rule.Children {
			result, err := e.evaluateRule(&rule.Children[i], data)
			if err != nil {
				return false, err
			}
			if result {
				return true, nil
			}
		}
		return false, nil

	default:
		return false, fmt.Errorf("unsupported logical operator: %s", rule.Operator)
	}
}

// extractValue extracts a value from data using a path (supports dot notation and bracket notation)
func (e *Evaluator) extractValue(data map[string]any, path string) (any, error) {
	// Remove leading dot
	path = strings.TrimPrefix(path, ".")

	// Handle simple paths (no nesting)
	if !strings.Contains(path, ".") && !strings.Contains(path, "[") {
		return data[path], nil
	}

	// Parse path segments
	segments := e.parsePath(path)
	current := any(data)

	for _, segment := range segments {
		currentMap, ok := current.(map[string]any)
		if !ok {
			return nil, nil
		}

		current = currentMap[segment]
		if current == nil {
			return nil, nil
		}
	}

	return current, nil
}

// parsePath parses a path into segments, handling dot notation and bracket notation
func (e *Evaluator) parsePath(path string) []string {
	segments := []string{}
	current := ""
	inBracket := false
	quoteChar := rune(0)

	for i, char := range path {
		switch {
		case char == '[' && !inBracket:
			if current != "" {
				segments = append(segments, current)
				current = ""
			}
			inBracket = true

		case char == ']' && inBracket:
			if current != "" {
				segments = append(segments, current)
				current = ""
			}
			inBracket = false
			quoteChar = 0

		case (char == '\'' || char == '"') && inBracket:
			if quoteChar == 0 {
				quoteChar = char
			} else if quoteChar == char {
				quoteChar = 0
			} else {
				current += string(char)
			}

		case char == '.' && !inBracket:
			if current != "" {
				segments = append(segments, current)
				current = ""
			}

		default:
			// Skip dots after closing brackets
			if !(char == '.' && i > 0 && path[i-1] == ']') {
				current += string(char)
			}
		}
	}

	if current != "" {
		segments = append(segments, current)
	}

	return segments
}

// evaluateComparison evaluates a comparison operation
func (e *Evaluator) evaluateComparison(actualValue any, operator rules.Operator, expectedValue any, ruleType rules.RuleType) (bool, error) {
	switch operator {
	case rules.OperatorIsSet:
		return actualValue != nil, nil

	case rules.OperatorIsNotSet:
		return actualValue == nil, nil

	case rules.OperatorEmpty:
		if actualValue == nil {
			return true, nil
		}
		if str, ok := actualValue.(string); ok {
			return str == "", nil
		}
		if arr, ok := actualValue.([]any); ok {
			return len(arr) == 0, nil
		}
		return false, nil
	}

	// For other operators, both values must exist
	if actualValue == nil || expectedValue == nil {
		return false, nil
	}

	// Type-specific comparisons
	switch ruleType {
	case rules.RuleTypeNumber:
		return e.compareNumbers(actualValue, operator, expectedValue)
	case rules.RuleTypeBoolean:
		return e.compareBooleans(actualValue, operator, expectedValue)
	case rules.RuleTypeDate:
		return e.compareDates(actualValue, operator, expectedValue)
	case rules.RuleTypeString:
		return e.compareStrings(actualValue, operator, expectedValue)
	case rules.RuleTypeArray:
		return e.compareArrays(actualValue, operator, expectedValue)
	default:
		return false, fmt.Errorf("unsupported rule type: %s", ruleType)
	}
}

// compareNumbers compares numeric values
func (e *Evaluator) compareNumbers(actual any, operator rules.Operator, expected any) (bool, error) {
	actualNum, err := toFloat64(actual)
	if err != nil {
		return false, err
	}

	expectedNum, err := toFloat64(expected)
	if err != nil {
		return false, err
	}

	switch operator {
	case rules.OperatorEquals:
		return actualNum == expectedNum, nil
	case rules.OperatorNotEquals:
		return actualNum != expectedNum, nil
	case rules.OperatorLessThan:
		return actualNum < expectedNum, nil
	case rules.OperatorLessEqual:
		return actualNum <= expectedNum, nil
	case rules.OperatorGreaterThan:
		return actualNum > expectedNum, nil
	case rules.OperatorGreaterEqual:
		return actualNum >= expectedNum, nil
	default:
		return false, fmt.Errorf("unsupported numeric operator: %s", operator)
	}
}

// compareBooleans compares boolean values
func (e *Evaluator) compareBooleans(actual any, operator rules.Operator, expected any) (bool, error) {
	actualBool, err := toBool(actual)
	if err != nil {
		return false, fmt.Errorf("actual value is not a boolean")
	}

	expectedBool, err := toBool(expected)
	if err != nil {
		return false, fmt.Errorf("expected value is not a boolean")
	}

	switch operator {
	case rules.OperatorEquals:
		return actualBool == expectedBool, nil
	case rules.OperatorNotEquals:
		return actualBool != expectedBool, nil
	default:
		return false, fmt.Errorf("unsupported boolean operator: %s", operator)
	}
}

// toBool converts a value to a boolean, handling both native bools and string
// representations. This is needed because JSON round-tripping through databases
// (e.g. PostgreSQL JSONB) can turn boolean values into strings.
func toBool(v any) (bool, error) {
	switch val := v.(type) {
	case bool:
		return val, nil
	case string:
		return strconv.ParseBool(val)
	default:
		return false, fmt.Errorf("cannot convert %T to boolean", v)
	}
}

// compareDates compares date/time values
func (e *Evaluator) compareDates(actual any, operator rules.Operator, expected any) (bool, error) {
	actualTime, err := toTime(actual)
	if err != nil {
		return false, err
	}

	expectedTime, err := toTime(expected)
	if err != nil {
		return false, err
	}

	switch operator {
	case rules.OperatorEquals:
		return actualTime.Equal(expectedTime), nil
	case rules.OperatorNotEquals:
		return !actualTime.Equal(expectedTime), nil
	case rules.OperatorLessThan:
		return actualTime.Before(expectedTime), nil
	case rules.OperatorLessEqual:
		return actualTime.Before(expectedTime) || actualTime.Equal(expectedTime), nil
	case rules.OperatorGreaterThan:
		return actualTime.After(expectedTime), nil
	case rules.OperatorGreaterEqual:
		return actualTime.After(expectedTime) || actualTime.Equal(expectedTime), nil
	case rules.OperatorIsSameDay:
		actualYear, actualMonth, actualDay := actualTime.Date()
		expectedYear, expectedMonth, expectedDay := expectedTime.Date()
		return actualYear == expectedYear && actualMonth == expectedMonth && actualDay == expectedDay, nil
	default:
		return false, fmt.Errorf("unsupported date operator: %s", operator)
	}
}

// compareStrings compares string values
func (e *Evaluator) compareStrings(actual any, operator rules.Operator, expected any) (bool, error) {
	actualStr, ok := actual.(string)
	if !ok {
		actualStr = fmt.Sprint(actual)
	}

	expectedStr, ok := expected.(string)
	if !ok {
		expectedStr = fmt.Sprint(expected)
	}

	// Case-insensitive comparison
	actualLower := strings.ToLower(actualStr)
	expectedLower := strings.ToLower(expectedStr)

	switch operator {
	case rules.OperatorEquals:
		return actualLower == expectedLower, nil
	case rules.OperatorNotEquals:
		return actualLower != expectedLower, nil
	case rules.OperatorContains:
		return strings.Contains(actualLower, expectedLower), nil
	case rules.OperatorNotContain:
		return !strings.Contains(actualLower, expectedLower), nil
	case rules.OperatorStartsWith:
		return strings.HasPrefix(actualLower, expectedLower), nil
	case rules.OperatorNotStartWith:
		return !strings.HasPrefix(actualLower, expectedLower), nil
	case rules.OperatorEndsWith:
		return strings.HasSuffix(actualLower, expectedLower), nil
	default:
		return false, fmt.Errorf("unsupported string operator: %s", operator)
	}
}

// compareArrays compares array values
func (e *Evaluator) compareArrays(actual any, operator rules.Operator, expected any) (bool, error) {
	actualArr, ok := actual.([]any)
	if !ok {
		return false, fmt.Errorf("actual value is not an array")
	}

	switch operator {
	case rules.OperatorAny:
		// Check if expected value exists in array
		for _, item := range actualArr {
			if item == expected {
				return true, nil
			}
		}
		return false, nil

	case rules.OperatorNone:
		// Check if expected value does not exist in array
		for _, item := range actualArr {
			if item == expected {
				return false, nil
			}
		}
		return true, nil

	default:
		return false, fmt.Errorf("unsupported array operator: %s", operator)
	}
}

// toFloat64 converts various numeric types to float64.
// Strings are parsed as numbers since rule values from JSON are often stored as strings.
func toFloat64(value any) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot convert string %q to float64: %w", v, err)
		}
		return f, nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", value)
	}
}

// toTime converts various time representations to time.Time
func toTime(value any) (time.Time, error) {
	switch v := value.(type) {
	case time.Time:
		return v, nil
	case string:
		// Try parsing common formats
		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02",
			"2006-01-02 15:04:05",
		}
		for _, format := range formats {
			if t, err := time.Parse(format, v); err == nil {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("cannot parse time string: %s", v)
	default:
		return time.Time{}, fmt.Errorf("cannot convert %T to time.Time", value)
	}
}
