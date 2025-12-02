package query

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lunogram/platform/services/nexus/internal/rules"
)

// pathSegmentRegex matches path segments in both dot and bracket notation
// Group 1: dot notation identifier (.field)
// Group 2: bracket notation with single quotes (['field'])
// Group 3: bracket notation with double quotes (["field"])
var pathSegmentRegex = regexp.MustCompile(`\.([a-zA-Z_][a-zA-Z0-9_]*)|\.?\['([^']+)'\]|\.?\["([^"]+)"\]`)

// buildRule recursively builds SQL conditions from a rule
func (qb *QueryBuilder) buildRule(rule *rules.Rule) (string, error) {
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

	for _, child := range rule.Children {
		condition, err := qb.buildRule(&child)
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
	column := qb.buildColumnPath("u", rule.Path, rule.Type)
	return qb.buildComparison(column, rule.Operator, rule.Value, rule.Type)
}

// buildColumnPath converts a dot-notation or bracket-notation path to PostgreSQL column reference
// Supports:
//   - Simple paths: ".email" -> "u.email"
//   - Nested paths: ".data.subscription.tier" -> "(u.data->'subscription'->>'tier')::text"
//   - Bracket notation: ".data['purchase agreement'].value" -> "(u.data->'purchase agreement'->>'value')::text"
//   - Mixed notation: ".data['user info'].name" -> "(u.data->'user info'->>'name')::text"
//   - Type casting: RuleTypeNumber -> "(u.data->>'count')::numeric"
func (qb *QueryBuilder) buildColumnPath(tableAlias, path string, ruleType rules.RuleType) string {
	matches := pathSegmentRegex.FindAllStringSubmatch(path, -1)
	if len(matches) == 0 {
		return tableAlias + "." + strings.TrimPrefix(path, ".")
	}

	result := tableAlias + "." + qb.extractKey(matches[0])

	// Build JSONB path accessors
	// Use -> for intermediate keys and ->> for the final key (text extraction)
	for i := 1; i < len(matches); i++ {
		operator := "->"
		if i == len(matches)-1 {
			operator = "->>"
		}
		result += operator + qb.quoteJSONBKey(qb.extractKey(matches[i]))
	}

	// Apply type cast for JSONB paths (when we have nested access)
	if len(matches) > 1 {
		result = qb.wrapWithTypeCast(result, ruleType)
	}

	return result
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

// quoteJSONBKey wraps a JSONB key in single quotes, escaping any single quotes within
func (qb *QueryBuilder) quoteJSONBKey(key string) string {
	// Escape single quotes by doubling them
	escaped := strings.ReplaceAll(key, "'", "''")
	return "'" + escaped + "'"
}

// wrapWithTypeCast wraps a JSONB text extraction with the appropriate PostgreSQL type cast
func (qb *QueryBuilder) wrapWithTypeCast(column string, ruleType rules.RuleType) string {
	return fmt.Sprintf("(%s)::%s", column, ruleType.SQL())
}
