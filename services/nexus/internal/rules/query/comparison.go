package query

import (
	"fmt"

	"github.com/lunogram/platform/services/nexus/internal/rules"
)

// buildComparison builds SQL comparison expressions based on operator and type
func (qb *QueryBuilder) buildComparison(column string, operator rules.Operator, value any, ruleType rules.RuleType) (string, error) {
	switch operator {
	case rules.OperatorIsSet:
		return fmt.Sprintf("%s IS NOT NULL", column), nil
	case rules.OperatorIsNotSet:
		return fmt.Sprintf("%s IS NULL", column), nil
	case rules.OperatorEmpty:
		if ruleType == rules.RuleTypeArray {
			return fmt.Sprintf("(%s IS NULL OR array_length(%s, 1) IS NULL)", column, column), nil
		}

		return fmt.Sprintf("(%s IS NULL OR %s = '')", column, column), nil
	}

	if value == nil {
		return "", fmt.Errorf("value is required for operator %s", operator)
	}

	switch operator {
	case rules.OperatorEquals:
		return fmt.Sprintf("%s = %s", column, qb.arg(value)), nil

	case rules.OperatorNotEquals:
		return fmt.Sprintf("%s != %s", column, qb.arg(value)), nil

	case rules.OperatorLessThan:
		return fmt.Sprintf("%s < %s", column, qb.arg(value)), nil

	case rules.OperatorLessEqual:
		return fmt.Sprintf("%s <= %s", column, qb.arg(value)), nil

	case rules.OperatorGreaterThan:
		return fmt.Sprintf("%s > %s", column, qb.arg(value)), nil

	case rules.OperatorGreaterEqual:
		return fmt.Sprintf("%s >= %s", column, qb.arg(value)), nil

	// String operators
	case rules.OperatorContains:
		return fmt.Sprintf("%s ILIKE %s", column, qb.arg(fmt.Sprintf("%%%s%%", value))), nil

	case rules.OperatorNotContain:
		return fmt.Sprintf("%s NOT ILIKE %s", column, qb.arg(fmt.Sprintf("%%%s%%", value))), nil

	case rules.OperatorStartsWith:
		return fmt.Sprintf("%s ILIKE %s", column, qb.arg(fmt.Sprintf("%s%%", value))), nil

	case rules.OperatorNotStartWith:
		return fmt.Sprintf("%s NOT ILIKE %s", column, qb.arg(fmt.Sprintf("%s%%", value))), nil

	case rules.OperatorEndsWith:
		return fmt.Sprintf("%s ILIKE %s", column, qb.arg(fmt.Sprintf("%%%s", value))), nil

	// Array operators
	case rules.OperatorAny:
		return fmt.Sprintf("%s = ANY(%s)", column, qb.arg(value)), nil

	case rules.OperatorNone:
		return fmt.Sprintf("%s != ALL(%s)", column, qb.arg(value)), nil

	case rules.OperatorIsSameDay:
		return fmt.Sprintf("DATE(%s) = DATE(%s)", column, qb.arg(value)), nil

	default:
		return "", fmt.Errorf("unsupported operator: %s", operator)
	}
}
