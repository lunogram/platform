package query

import (
	"fmt"
	"strings"

	"github.com/lunogram/platform/internal/rules"
)

// organizationDirectColumns are columns that exist directly on the organizations table
// and should NOT be normalized to the JSONB data column.
var organizationDirectColumns = map[string]bool{
	".name":        true,
	".external_id": true,
	".created_at":  true,
	".updated_at":  true,
}

// normalizeOrganizationPath normalizes a path for organization queries.
// Direct columns (name, external_id, etc.) are left as-is, while other
// paths are normalized to access the JSONB data column.
func normalizeOrganizationPath(path string) string {
	if organizationDirectColumns[path] {
		return path
	}
	return normalizeDataPath(path)
}

// buildOrganizationRule builds SQL for organization attribute rules.
// This filters users who belong to organizations matching the criteria.
// Example: "get all users in organizations where data.tier = 'gold'"
func (qb *QueryBuilder) buildOrganizationRule(rule *rules.Rule) (string, error) {
	path := normalizeOrganizationPath(rule.Path)
	condition, err := qb.buildConditionFromPath("o", path, rule)
	if err != nil {
		return "", err
	}

	subquery := qb.buildOrgUserSubquery(condition, "")
	qb.addJoinForUserIDs(subquery)

	return "", nil
}

// buildOrganizationUserRule builds SQL for organization user attribute rules.
// This filters users based on their data within the organization_users table.
// Example: "get all users who have role = 'admin' in any organization"
func (qb *QueryBuilder) buildOrganizationUserRule(rule *rules.Rule) (string, error) {
	path := normalizeDataPath(rule.Path)
	condition, err := qb.buildConditionFromPath("ou", path, rule)
	if err != nil {
		return "", err
	}

	subquery := qb.buildOrgUserSubquery(condition, "")
	qb.addJoinForUserIDs(subquery)

	return "", nil
}

// buildOrganizationPropertyRule builds SQL for organization property wrapper rules.
// This filters users who belong to organizations matching property conditions,
// with optional filtering on which members to include (user_match).
// Example: "get users in organizations where tier = 'gold', only including admin members"
func (qb *QueryBuilder) buildOrganizationPropertyRule(rule *rules.Rule) (string, error) {
	orgCondition, err := qb.collectOrganizationConditions(rule)
	if err != nil {
		return "", err
	}
	if orgCondition == "" {
		return "", nil
	}

	memberCondition, err := qb.extractMemberConditions(rule)
	if err != nil {
		return "", err
	}

	subquery := qb.buildOrgUserSubquery(orgCondition, memberCondition)
	qb.addJoinForUserIDs(subquery)

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

	orgConditions, orgUserConditions, err := qb.collectOrgAndOrgUserConditions(rule)
	if err != nil {
		return "", err
	}

	allConditions := append(orgConditions, orgUserConditions...)
	if len(allConditions) == 0 {
		return "", nil
	}

	combinedCondition := joinConditions(allConditions, rule.Operator.SQL())
	subquery := qb.buildOrgUserSubquery(combinedCondition, "")
	qb.addJoinForUserIDs(subquery)

	return "", nil
}

// buildOrganizationEventRule builds SQL for organization event rules with frequency.
// This finds users who belong to organizations that have performed certain events,
// with optional filtering on which members to include.
func (qb *QueryBuilder) buildOrganizationEventRule(rule *rules.Rule) (string, error) {
	if rule.Frequency == nil {
		return "", fmt.Errorf("organization event rule requires frequency")
	}

	eventConditions, err := qb.buildOrgEventConditions(rule)
	if err != nil {
		return "", err
	}

	havingClause, err := qb.buildHavingClause(rule.Frequency)
	if err != nil {
		return "", err
	}

	memberCondition, err := qb.extractMemberConditions(rule)
	if err != nil {
		return "", err
	}

	subquery := qb.buildOrgEventSubquery(eventConditions, havingClause, memberCondition)
	qb.addJoinForUserIDs(subquery)

	return "", nil
}

// buildMemberConditions builds SQL conditions for filtering organization members
// based on their membership data (organization_users.data).
func (qb *QueryBuilder) buildMemberConditions(rule *rules.Rule) (string, error) {
	if rule == nil {
		return "", nil
	}

	if rule.IsWrapper() {
		return qb.buildMemberConditionsWrapper(rule)
	}

	return qb.buildSingleMemberCondition(rule)
}

// =============================================================================
// Helper methods for building subqueries
// =============================================================================

// buildOrgUserSubquery builds a subquery that selects user_ids from organization_users
// filtered by organization conditions and optional member conditions.
func (qb *QueryBuilder) buildOrgUserSubquery(orgCondition, memberCondition string) string {
	whereClause := fmt.Sprintf("o.project_id = %s AND %s", qb.arg(qb.projectID), orgCondition)

	if memberCondition != "" {
		whereClause += " AND " + memberCondition
	}

	return fmt.Sprintf("SELECT DISTINCT ou.user_id FROM organization_users ou JOIN organizations o ON o.id = ou.organization_id WHERE %s", whereClause)
}

// buildOrgEventSubquery builds a subquery that finds users in organizations
// matching event frequency criteria, optionally filtered by member conditions.
func (qb *QueryBuilder) buildOrgEventSubquery(eventConditions []string, havingClause, memberCondition string) string {
	matchingOrgsSubquery := fmt.Sprintf(
		"SELECT oe.organization_id FROM organization_events oe JOIN events e ON e.id = oe.event_id WHERE %s GROUP BY oe.organization_id HAVING %s",
		strings.Join(eventConditions, " AND "),
		havingClause)

	if memberCondition != "" {
		return fmt.Sprintf(
			"SELECT DISTINCT ou.user_id FROM organization_users ou JOIN (%s) matching_orgs ON matching_orgs.organization_id = ou.organization_id WHERE %s",
			matchingOrgsSubquery,
			memberCondition)
	}

	return fmt.Sprintf(
		"SELECT DISTINCT ou.user_id FROM organization_users ou JOIN (%s) matching_orgs ON matching_orgs.organization_id = ou.organization_id",
		matchingOrgsSubquery)
}

// collectOrganizationConditions extracts and combines organization property conditions
// from a rule's children.
func (qb *QueryBuilder) collectOrganizationConditions(rule *rules.Rule) (string, error) {
	if !rule.HasChildren() {
		return "", nil
	}

	conditions := make([]string, 0, len(rule.Children))

	for i := range rule.Children {
		child := &rule.Children[i]
		if child.Group != rules.RuleGroupOrganization {
			continue
		}

		path := normalizeOrganizationPath(child.Path)
		condition, err := qb.buildConditionFromPath("o", path, child)
		if err != nil {
			return "", err
		}
		conditions = append(conditions, condition)
	}

	return joinConditions(conditions, rule.Operator.SQL()), nil
}

// collectOrgAndOrgUserConditions separates and builds conditions for both
// organization and organization_user groups from a rule's children.
func (qb *QueryBuilder) collectOrgAndOrgUserConditions(rule *rules.Rule) ([]string, []string, error) {
	orgConditions := make([]string, 0)
	orgUserConditions := make([]string, 0)

	for i := range rule.Children {
		child := &rule.Children[i]

		switch child.Group {
		case rules.RuleGroupOrganization:
			path := normalizeOrganizationPath(child.Path)
			condition, err := qb.buildConditionFromPath("o", path, child)
			if err != nil {
				return nil, nil, err
			}
			orgConditions = append(orgConditions, condition)

		case rules.RuleGroupOrganizationUser:
			path := normalizeDataPath(child.Path)
			condition, err := qb.buildConditionFromPath("ou", path, child)
			if err != nil {
				return nil, nil, err
			}
			orgUserConditions = append(orgUserConditions, condition)
		}
	}

	return orgConditions, orgUserConditions, nil
}

// buildOrgEventConditions builds the list of conditions for organization event queries.
func (qb *QueryBuilder) buildOrgEventConditions(rule *rules.Rule) ([]string, error) {
	interval, err := qb.buildRollingPeriodInterval(rule.Frequency.Period)
	if err != nil {
		return nil, err
	}

	conditions := []string{
		fmt.Sprintf("oe.created_at >= NOW() - %s::interval", qb.arg(interval)),
	}

	if rule.Value != nil {
		conditions = append(conditions, fmt.Sprintf("e.name = %s", qb.arg(rule.Value)))
	}

	if rule.HasChildren() {
		childConditions, err := qb.collectOrgEventChildConditions(rule)
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, childConditions...)
	}

	return conditions, nil
}

// collectOrgEventChildConditions extracts event property conditions from a rule's children.
func (qb *QueryBuilder) collectOrgEventChildConditions(rule *rules.Rule) ([]string, error) {
	conditions := make([]string, 0)

	for i := range rule.Children {
		child := &rule.Children[i]

		if !isEventPropertyGroup(child.Group) {
			continue
		}

		path := normalizeDataPath(child.Path)
		condition, err := qb.buildConditionFromPath("oe", path, child)
		if err != nil {
			return nil, err
		}
		conditions = append(conditions, condition)
	}

	return conditions, nil
}

// isEventPropertyGroup returns true if the group can contain event property conditions.
func isEventPropertyGroup(group rules.RuleGroup) bool {
	return group == rules.RuleGroupEvent ||
		group == rules.RuleGroupOrganizationEvent ||
		group == rules.RuleGroupParent
}

// extractMemberConditions extracts member filter conditions from a rule's UserMatch field.
func (qb *QueryBuilder) extractMemberConditions(rule *rules.Rule) (string, error) {
	if rule.UserMatch == nil {
		return "", nil
	}
	if rule.UserMatch.Type != rules.UserMatchConditions {
		return "", nil
	}
	if rule.UserMatch.MemberConditions == nil {
		return "", nil
	}

	return qb.buildMemberConditions(rule.UserMatch.MemberConditions)
}

// buildMemberConditionsWrapper handles wrapper rules in member conditions recursively.
func (qb *QueryBuilder) buildMemberConditionsWrapper(rule *rules.Rule) (string, error) {
	if !rule.HasChildren() {
		return "", nil
	}

	conditions := make([]string, 0, len(rule.Children))

	for i := range rule.Children {
		child := &rule.Children[i]
		condition, err := qb.buildMemberConditions(child)
		if err != nil {
			return "", err
		}
		if condition != "" {
			conditions = append(conditions, condition)
		}
	}

	return joinConditions(conditions, rule.Operator.SQL()), nil
}

// buildSingleMemberCondition builds a condition for a single member property rule.
// Member conditions filter on organization_users.data JSONB column.
func (qb *QueryBuilder) buildSingleMemberCondition(rule *rules.Rule) (string, error) {
	path := normalizeDataPath(rule.Path)
	return qb.buildConditionFromPath("ou", path, rule)
}
