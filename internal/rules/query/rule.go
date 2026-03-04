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

// normalizeDataPath ensures a path accesses the JSONB data column.
// Paths like ".tier" become ".data.tier" to access the JSONB data column,
// while paths already starting with ".data" as the first segment are returned unchanged.
// This correctly handles paths like ".database" (normalized to ".data.database")
// vs ".data.tier" (already normalized, unchanged).
func normalizeDataPath(path string) string {
	// Check if path already starts with ".data"
	if path == ".data" || strings.HasPrefix(path, ".data.") || strings.HasPrefix(path, ".data[") {
		return path
	}

	return ".data" + path
}

// joinConditions combines SQL conditions with a logical operator.
// Returns empty string if no conditions, the single condition if only one,
// or parenthesized conditions joined by the operator if multiple.
func joinConditions(conditions []string, operator string) string {
	switch len(conditions) {
	case 0:
		return ""
	case 1:
		return conditions[0]
	default:
		return "(" + strings.Join(conditions, " "+operator+" ") + ")"
	}
}

// buildHavingClause generates a SQL HAVING clause for frequency comparisons.
func (qb *QueryBuilder) buildHavingClause(freq *rules.Frequency) (string, error) {
	countArg := qb.arg(freq.Count)

	switch freq.Operator {
	case rules.OperatorGreaterThan:
		return fmt.Sprintf("COUNT(*) > %s", countArg), nil
	case rules.OperatorGreaterEqual:
		return fmt.Sprintf("COUNT(*) >= %s", countArg), nil
	case rules.OperatorLessThan:
		return fmt.Sprintf("COUNT(*) < %s", countArg), nil
	case rules.OperatorLessEqual:
		return fmt.Sprintf("COUNT(*) <= %s", countArg), nil
	case rules.OperatorEquals:
		return fmt.Sprintf("COUNT(*) = %s", countArg), nil
	case rules.OperatorNotEquals:
		return fmt.Sprintf("COUNT(*) != %s", countArg), nil
	default:
		return "", fmt.Errorf("unsupported frequency operator: %s", freq.Operator)
	}
}

// buildRollingPeriodInterval generates a PostgreSQL interval string for rolling periods.
func (qb *QueryBuilder) buildRollingPeriodInterval(period rules.Period) (string, error) {
	if period.Type != rules.PeriodTypeRolling {
		return "", fmt.Errorf("only rolling periods are currently supported")
	}
	return fmt.Sprintf("%d %s", period.Value, period.Unit.SQL()), nil
}

// buildConditionFromPath builds a SQL condition using a custom path (e.g., normalized data path).
func (qb *QueryBuilder) buildConditionFromPath(tableAlias, path string, rule *rules.Rule) (string, error) {
	column, err := qb.buildColumnPath(tableAlias, path, rule.Type)
	if err != nil {
		return "", err
	}
	return qb.buildComparison(column, rule.Operator, rule.Value, rule.Type)
}

// addJoinForUserIDs adds a JOIN clause that filters users by a subquery returning user_id.
func (qb *QueryBuilder) addJoinForUserIDs(subquery string) {
	alias := qb.nextJoinAlias()
	joinClause := fmt.Sprintf("JOIN (%s) %s ON %s.user_id = u.id", subquery, alias, alias)
	qb.joins = append(qb.joins, joinClause)
}

// buildRule recursively builds SQL conditions from a rule
func (qb *QueryBuilder) buildRule(rule *rules.Rule) (string, error) {
	// Check if this is an event wrapper with frequency - treat it as an event rule
	if rule.IsWrapper() && rule.Group == rules.RuleGroupEvent && rule.Frequency != nil {
		return qb.buildEventRule(rule)
	}

	// Check if this is an organization event rule
	if rule.IsWrapper() && rule.Group == rules.RuleGroupOrganizationEvent && rule.Frequency != nil {
		return qb.buildOrganizationEventRule(rule)
	}

	// Check if this is an organization property rule (wrapper with group "organization")
	if rule.IsWrapper() && rule.Group == rules.RuleGroupOrganization {
		return qb.buildOrganizationPropertyRule(rule)
	}

	// Check if this is an organization wrapper (contains both org and org_user rules)
	if rule.IsWrapper() && qb.isOrganizationWrapper(rule) {
		return qb.buildOrganizationWrapperRule(rule)
	}

	if rule.IsWrapper() {
		return qb.buildWrapper(rule)
	}

	switch rule.Group {
	case rules.RuleGroupUser:
		return qb.buildUserRule(rule)
	case rules.RuleGroupEvent:
		return qb.buildEventRule(rule)
	case rules.RuleGroupOrganization:
		return qb.buildOrganizationRule(rule)
	case rules.RuleGroupOrganizationUser:
		return qb.buildOrganizationUserRule(rule)
	case rules.RuleGroupOrganizationEvent:
		return qb.buildOrganizationEventRule(rule)
	default:
		return "", fmt.Errorf("unsupported rule group: %s", rule.Group)
	}
}

// isOrganizationWrapper checks if a wrapper rule contains only organization-related rules
// (RuleGroupOrganization and/or RuleGroupOrganizationUser). Nested wrappers (RuleGroupParent)
// are not allowed here - they should be handled by buildWrapper which recurses properly.
func (qb *QueryBuilder) isOrganizationWrapper(rule *rules.Rule) bool {
	if !rule.HasChildren() {
		return false
	}

	for _, child := range rule.Children {
		if child.Group != rules.RuleGroupOrganization && child.Group != rules.RuleGroupOrganizationUser {
			// If there's any non-organization rule (including nested wrappers), this is not a pure org wrapper
			return false
		}
	}

	return true
}

// buildWrapper builds SQL for wrapper nodes with logical operators
func (qb *QueryBuilder) buildWrapper(rule *rules.Rule) (string, error) {
	if !rule.HasChildren() {
		return "", nil
	}

	// For OR conditions with join-producing children, we need special handling
	// to combine joins with UNION instead of INNER JOINs
	if rule.Operator == rules.OperatorOr && qb.hasJoinProducingChildren(rule) {
		return qb.buildOrWrapper(rule)
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

	return joinConditions(conditions, rule.Operator.SQL()), nil
}

// hasJoinProducingChildren checks if any child rule will produce JOINs
// (event, organization, organization_event, organization_user rules produce JOINs)
func (qb *QueryBuilder) hasJoinProducingChildren(rule *rules.Rule) bool {
	for _, child := range rule.Children {
		if qb.isJoinProducingRule(&child) {
			return true
		}
	}
	return false
}

// isJoinProducingRule checks if a rule produces a JOIN instead of a WHERE condition
func (qb *QueryBuilder) isJoinProducingRule(rule *rules.Rule) bool {
	// Event rules with frequency
	if rule.IsWrapper() && rule.Group == rules.RuleGroupEvent && rule.Frequency != nil {
		return true
	}
	// Organization event rules
	if rule.IsWrapper() && rule.Group == rules.RuleGroupOrganizationEvent && rule.Frequency != nil {
		return true
	}
	// Organization property rules (wrapper with group "organization")
	if rule.IsWrapper() && rule.Group == rules.RuleGroupOrganization {
		return true
	}
	// Non-wrapper event rules
	if rule.Group == rules.RuleGroupEvent {
		return true
	}
	// Non-wrapper organization rules
	if rule.Group == rules.RuleGroupOrganization {
		return true
	}
	// Organization user rules
	if rule.Group == rules.RuleGroupOrganizationUser {
		return true
	}
	// Organization event rules
	if rule.Group == rules.RuleGroupOrganizationEvent {
		return true
	}
	return false
}

// buildOrWrapper builds SQL for OR wrapper with join-producing children
// It combines all children into user_id subqueries and UNIONs them to achieve true OR semantics.
// This ensures users matching ANY branch are included, regardless of whether the branch
// produces a JOIN or a WHERE condition.
func (qb *QueryBuilder) buildOrWrapper(rule *rules.Rule) (string, error) {
	// Collect all subqueries (both from join-producing and non-join-producing children)
	subqueries := []string{}

	// Save current join count to detect new joins
	startJoinCount := len(qb.joins)

	for i := range rule.Children {
		child := &rule.Children[i]

		if qb.isJoinProducingRule(child) {
			// Build the child rule which will add to qb.joins
			joinsBefore := len(qb.joins)
			_, err := qb.buildRule(child)
			if err != nil {
				return "", err
			}

			// Extract the subquery from the newly added join
			if len(qb.joins) > joinsBefore {
				// Get the last added join and extract the subquery
				lastJoin := qb.joins[len(qb.joins)-1]
				subquery := qb.extractSubqueryFromJoin(lastJoin)
				if subquery != "" {
					subqueries = append(subqueries, subquery)
				}
			}
		} else {
			// For non-join-producing rules (e.g., user attributes), convert to a subquery
			// so it can be UNIONed with join-producing subqueries
			condition, err := qb.buildRule(child)
			if err != nil {
				return "", err
			}
			if condition != "" {
				// Build a subquery that returns user_id for users matching this condition
				subquery := fmt.Sprintf("SELECT u.id AS user_id FROM users u WHERE u.project_id = %s AND %s", qb.arg(qb.projectID), condition)
				subqueries = append(subqueries, subquery)
			}
		}
	}

	// Remove all joins that were added by children (we'll combine them)
	qb.joins = qb.joins[:startJoinCount]

	// Combine all subqueries with UNION into a single JOIN
	if len(subqueries) > 0 {
		alias := qb.nextJoinAlias()
		unionQuery := strings.Join(subqueries, " UNION ")
		joinClause := fmt.Sprintf("JOIN (%s) %s ON %s.user_id = u.id", unionQuery, alias, alias)
		qb.joins = append(qb.joins, joinClause)
	}

	// No WHERE conditions needed - all branches are in the UNION
	return "", nil
}

// extractSubqueryFromJoin extracts the subquery part from a JOIN clause
// JOIN (SELECT ... ) alias ON ... -> SELECT ...
func (qb *QueryBuilder) extractSubqueryFromJoin(joinClause string) string {
	// Find the opening parenthesis after JOIN
	start := strings.Index(joinClause, "(")
	if start == -1 {
		return ""
	}

	// Find the matching closing parenthesis
	depth := 0
	for i := start; i < len(joinClause); i++ {
		if joinClause[i] == '(' {
			depth++
		} else if joinClause[i] == ')' {
			depth--
			if depth == 0 {
				return joinClause[start+1 : i]
			}
		}
	}

	return ""
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
