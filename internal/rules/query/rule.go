package query

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/lunogram/platform/internal/rules"
)

// pathSegmentRegex matches path segments:
// .field          - dot notation
// ['field']       - bracket with single quotes
// ["field"]       - bracket with double quotes
var pathSegmentRegex = regexp.MustCompile(`\.([a-zA-Z_][a-zA-Z0-9_]*)|\.?\['([^']+)'\]|\.?\["([^"]+)"\]`)

// validKeyPattern ensures JSONB keys contain only allowed characters
var validKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9_. -]+$`)

// buildRule recursively builds SQL conditions from a rule
func (qb *QueryBuilder) buildRule(rule *rules.Rule) (string, error) {
	// Check if this is an event wrapper with frequency - treat it as an event rule
	if rule.IsWrapper() && rule.Group == rules.RuleGroupEvent && rule.Frequency != nil {
		return qb.buildEventRule(rule)
	}

	if rule.IsWrapper() {
		return qb.buildWrapper(rule)
	}

	switch rule.Group {
	case rules.RuleGroupUser:
		return qb.buildUserRule(rule)
	case rules.RuleGroupEvent:
		return qb.buildEventRule(rule)
	default:
		return "", fmt.Errorf("unsupported rule group: %s", rule.Group)
	}
}

// buildWrapper builds SQL for wrapper nodes with logical operators
func (qb *QueryBuilder) buildWrapper(rule *rules.Rule) (string, error) {
	if !rule.HasChildren() {
		return "", nil
	}

	conditions := make([]string, 0, len(rule.Children))

	for i := range rule.Children {
		condition, err := qb.buildRule(&rule.Children[i])
		if err != nil {
			return "", err
		}
		if condition != "" {
			conditions = append(conditions, condition)
		}
	}

	if len(conditions) == 0 {
		return "", nil
	}

	if len(conditions) == 1 {
		return conditions[0], nil
	}

	logicalOp := rule.Operator.SQL()
	return "(" + strings.Join(conditions, " "+logicalOp+" ") + ")", nil
}

// buildUserRule builds SQL for user attribute rules
func (qb *QueryBuilder) buildUserRule(rule *rules.Rule) (string, error) {
	column, err := qb.buildColumnPath("u", rule.Path, rule.Type)
	if err != nil {
		return "", err
	}

	return qb.buildComparison(column, rule.Operator, rule.Value, rule.Type)
}

// buildColumnPath converts a dot-notation or bracket-notation path to PostgreSQL column reference
// Supports:
//   - Simple paths: ".email" -> "u.email"
//   - Nested paths: ".data.subscription.tier" -> "(u.data->'subscription'->>'tier')::text"
//   - Bracket notation: ".data['purchase agreement'].value" -> "(u.data->'purchase agreement'->>'value')::text"
//   - Mixed notation: ".data['user info'].name" -> "(u.data->'user info'->>'name')::text"
//   - Type casting: RuleTypeNumber -> "(u.data->>'count')::numeric"
func (qb *QueryBuilder) buildColumnPath(tableAlias, path string, ruleType rules.RuleType) (string, error) {
	matches := pathSegmentRegex.FindAllStringSubmatch(path, -1)
	if len(matches) == 0 {
		return tableAlias + "." + strings.TrimPrefix(path, "."), nil
	}

	result := tableAlias + "." + qb.extractKey(matches[0])

	// NOTE: build JSONB path accessors Use -> for intermediate keys and ->> for
	// the final key (text extraction)
	for i := 1; i < len(matches); i++ {
		operator := "->"
		if i == len(matches)-1 {
			operator = "->>"
		}

		key, err := qb.quoteJSONBKey(qb.extractKey(matches[i]))
		if err != nil {
			return "", err
		}

		result += operator + key
	}

	// NOTE: apply type cast for JSONB paths (when we have nested access)
	if len(matches) > 1 {
		result = qb.wrapWithTypeCast(result, ruleType)
	}

	return result, nil
}

// extractKey extracts the key from a regex match (whichever group matched)
func (qb *QueryBuilder) extractKey(match []string) string {
	for _, group := range match[1:] {
		if group != "" {
			return group
		}
	}
	return ""
}

// quoteJSONBKey safely quotes a JSONB key for use in PostgreSQL JSONB operators
// Only alphanumeric characters, underscores, hyphens, and dots are allowed.
func (qb *QueryBuilder) quoteJSONBKey(key string) (string, error) {
	if len(key) == 0 {
		return "", errors.New("JSONB key cannot be empty")
	}

	if !validKeyPattern.MatchString(key) {
		return "", fmt.Errorf("JSONB key contains invalid characters: %q (only alphanumeric, underscore, hyphen, and dot allowed)", key)
	}

	return fmt.Sprintf("'%s'", key), nil
}

// wrapWithTypeCast wraps a JSONB text extraction with the appropriate PostgreSQL type cast
func (qb *QueryBuilder) wrapWithTypeCast(column string, ruleType rules.RuleType) string {
	return fmt.Sprintf("(%s)::%s", column, ruleType.SQL())
}
