package query

import (
	"fmt"
	"strings"

	"github.com/lunogram/platform/internal/rules"
)

// buildOrganizationRule builds SQL for organization attribute rules.
// This filters users who belong to organizations matching the criteria.
// Example: "get all users in organizations where data.tier = 'gold'"
func (qb *QueryBuilder) buildOrganizationRule(rule *rules.Rule) (string, error) {
	column, err := qb.buildColumnPath("o", rule.Path, rule.Type)
	if err != nil {
		return "", err
	}

	condition, err := qb.buildComparison(column, rule.Operator, rule.Value, rule.Type)
	if err != nil {
		return "", err
	}

	// Generate a unique alias for this organization join
	alias := qb.nextJoinAlias()

	// Build the JOIN clause that connects users to organizations via organization_users
	// and filters by organization attributes
	joinClause := fmt.Sprintf(
		"JOIN (SELECT DISTINCT ou.user_id FROM organization_users ou JOIN organizations o ON o.id = ou.organization_id WHERE o.project_id = %s AND %s) %s ON %s.user_id = u.id",
		qb.arg(qb.projectID),
		condition,
		alias,
		alias,
	)

	qb.joins = append(qb.joins, joinClause)

	// Return empty condition since the filtering is in the JOIN
	return "", nil
}

// buildOrganizationUserRule builds SQL for organization user attribute rules.
// This filters users based on their data within the organization_users table.
// Example: "get all users who have role = 'admin' in any organization"
func (qb *QueryBuilder) buildOrganizationUserRule(rule *rules.Rule) (string, error) {
	column, err := qb.buildColumnPath("ou", rule.Path, rule.Type)
	if err != nil {
		return "", err
	}

	condition, err := qb.buildComparison(column, rule.Operator, rule.Value, rule.Type)
	if err != nil {
		return "", err
	}

	// Generate a unique alias for this organization user join
	alias := qb.nextJoinAlias()

	// Build the JOIN clause that filters users by their organization_users data
	joinClause := fmt.Sprintf(
		"JOIN (SELECT DISTINCT ou.user_id FROM organization_users ou JOIN organizations o ON o.id = ou.organization_id WHERE o.project_id = %s AND %s) %s ON %s.user_id = u.id",
		qb.arg(qb.projectID),
		condition,
		alias,
		alias,
	)

	qb.joins = append(qb.joins, joinClause)

	// Return empty condition since the filtering is in the JOIN
	return "", nil
}

// buildOrganizationWrapperRule builds SQL for wrapper rules that combine
// organization and organization user conditions. This is used when you need
// to match both organization attributes AND organization user attributes together.
// Example: "get users in orgs where tier = 'gold' AND user has role = 'admin'"
func (qb *QueryBuilder) buildOrganizationWrapperRule(rule *rules.Rule) (string, error) {
	if !rule.HasChildren() {
		return "", nil
	}

	// Collect conditions for organizations and organization users
	orgConditions := []string{}
	orgUserConditions := []string{}

	for i := range rule.Children {
		child := &rule.Children[i]

		switch child.Group {
		case rules.RuleGroupOrganization:
			column, err := qb.buildColumnPath("o", child.Path, child.Type)
			if err != nil {
				return "", err
			}
			condition, err := qb.buildComparison(column, child.Operator, child.Value, child.Type)
			if err != nil {
				return "", err
			}
			orgConditions = append(orgConditions, condition)

		case rules.RuleGroupOrganizationUser:
			column, err := qb.buildColumnPath("ou", child.Path, child.Type)
			if err != nil {
				return "", err
			}
			condition, err := qb.buildComparison(column, child.Operator, child.Value, child.Type)
			if err != nil {
				return "", err
			}
			orgUserConditions = append(orgUserConditions, condition)
		}
	}

	// Combine all conditions
	allConditions := append(orgConditions, orgUserConditions...)
	if len(allConditions) == 0 {
		return "", nil
	}

	logicalOp := rule.Operator.SQL()
	combinedCondition := strings.Join(allConditions, " "+logicalOp+" ")

	// Generate a unique alias for this combined join
	alias := qb.nextJoinAlias()

	// Build the JOIN clause that combines org and organization user conditions
	joinClause := fmt.Sprintf(
		"JOIN (SELECT DISTINCT ou.user_id FROM organization_users ou JOIN organizations o ON o.id = ou.organization_id WHERE o.project_id = %s AND (%s)) %s ON %s.user_id = u.id",
		qb.arg(qb.projectID),
		combinedCondition,
		alias,
		alias,
	)

	qb.joins = append(qb.joins, joinClause)

	// Return empty condition since the filtering is in the JOIN
	return "", nil
}
