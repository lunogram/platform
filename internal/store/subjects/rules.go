package subjects

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/store"
)

// durationToPostgresInterval converts a time.Duration to a PostgreSQL interval
// string (e.g. "3600 seconds"). Using seconds avoids ambiguity with calendar
// units like months.
func durationToPostgresInterval(d time.Duration) string {
	return fmt.Sprintf("%d seconds", int(d.Seconds()))
}

type Rule struct {
	ID                         uuid.UUID                  `db:"id"`
	ProjectID                  uuid.UUID                  `db:"project_id"`
	Rule                       store.JSONB[rules.RuleSet] `db:"rule"`
	DependsOnEvents            bool                       `db:"depends_on_events"`
	DependsOnUsers             bool                       `db:"depends_on_users"`
	DependsOnOrganizations     bool                       `db:"depends_on_organizations"`
	DependsOnOrganizationUsers bool                       `db:"depends_on_organization_users"`
	DependsOnTime              bool                       `db:"depends_on_time"`
	RecompileInterval          *string                    `db:"recompile_interval"`
	Events                     store.UUIDArray            `db:"events"`
	Version                    int                        `db:"version"`
	CreatedAt                  time.Time                  `db:"created_at"`
	UpdatedAt                  time.Time                  `db:"updated_at"`
}

func NewRulesStore(db store.DB) *RulesStore {
	return &RulesStore{
		db: db,
	}
}

type RulesStore struct {
	db store.DB
}

func (s *RulesStore) CreateOrUpdateRule(ctx context.Context, projectID uuid.UUID, id *uuid.UUID, rule rules.RuleSet) (uuid.UUID, error) {
	dependsOnTime := rule.DependsOnTime()
	var recompileInterval *string
	if ri := rule.RecompileInterval(); ri != nil {
		v := durationToPostgresInterval(*ri)
		recompileInterval = &v
	}

	if id != nil {
		err := s.UpdateRule(ctx, projectID, *id, RuleUpdate{
			Rule:                       &store.JSONB[rules.RuleSet]{Data: rule},
			DependsOnEvents:            rule.DependsOnEvents(),
			DependsOnUsers:             rule.DependsOnUsers(),
			DependsOnOrganizations:     rule.DependsOnOrganizations(),
			DependsOnOrganizationUsers: rule.DependsOnOrganizationUsers(),
			DependsOnTime:              dependsOnTime,
			RecompileInterval:          recompileInterval,
		})

		return *id, err
	}

	return s.CreateRule(ctx, Rule{
		ProjectID:                  projectID,
		Rule:                       store.JSONB[rules.RuleSet]{Data: rule},
		DependsOnEvents:            rule.DependsOnEvents(),
		DependsOnUsers:             rule.DependsOnUsers(),
		DependsOnOrganizations:     rule.DependsOnOrganizations(),
		DependsOnOrganizationUsers: rule.DependsOnOrganizationUsers(),
		DependsOnTime:              dependsOnTime,
		RecompileInterval:          recompileInterval,
		Version:                    1,
	})
}

func (s *RulesStore) CreateRule(ctx context.Context, rule Rule) (uuid.UUID, error) {
	stmt := `
	INSERT INTO rules (project_id, rule, depends_on_events, depends_on_users, depends_on_organizations, depends_on_organization_users, depends_on_time, recompile_interval, version)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8::interval, $9)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt, rule.ProjectID, rule.Rule, rule.DependsOnEvents, rule.DependsOnUsers, rule.DependsOnOrganizations, rule.DependsOnOrganizationUsers, rule.DependsOnTime, rule.RecompileInterval, rule.Version)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

// EventDependency represents an event name with its subject type for rule dependencies
type EventDependency struct {
	Name        string
	SubjectType SubjectType
}

func (s *RulesStore) SetRuleEventDependencies(ctx context.Context, projectID, ruleID uuid.UUID, events []EventDependency) error {
	// If no events provided, remove all existing dependencies for this rule
	if len(events) == 0 {
		_, err := s.db.ExecContext(ctx, `DELETE FROM rules_events WHERE rule_id = $1`, ruleID)
		return err
	}

	// Extract names and subject types into parallel arrays for SQL.
	// Note: These arrays must stay in sync - index i of names corresponds to index i of subjectTypes.
	names := make([]string, len(events))
	subjectTypes := make([]string, len(events))
	for i, e := range events {
		names[i] = e.Name
		subjectTypes[i] = string(e.SubjectType)
	}

	query := `
	WITH event_ids AS (
		SELECT e.id
		FROM events e
		WHERE e.project_id = $1
		AND e.deleted_at IS NULL
		AND EXISTS (
			SELECT 1
			FROM unnest($3::text[], $4::text[]) AS dep(name, subject_type)
			WHERE e.name = dep.name AND e.subject_type = dep.subject_type::subject_type
		)
	),
	deleted AS (
		DELETE FROM rules_events
		WHERE rule_id = $2
		AND event_id NOT IN (SELECT id FROM event_ids)
	)
	INSERT INTO rules_events (rule_id, event_id)
	SELECT $2, id FROM event_ids
	ON CONFLICT (rule_id, event_id) DO NOTHING`

	_, err := s.db.ExecContext(ctx, query, projectID, ruleID, pq.Array(names), pq.Array(subjectTypes))
	return err
}

func (s *RulesStore) GetRule(ctx context.Context, projectID, ruleID uuid.UUID) (*Rule, error) {
	query := `
	SELECT
		r.id,
		r.project_id,
		r.rule,
		r.depends_on_events,
		r.depends_on_users,
		r.depends_on_organizations,
		r.depends_on_organization_users,
		r.depends_on_time,
		r.recompile_interval,
		COALESCE(
			(
				SELECT array_agg(re.event_id)
				FROM rules_events re
				WHERE re.rule_id = r.id
			),
			'{}'
		) AS events,
		r.version,
		r.created_at,
		r.updated_at
	FROM rules r
	WHERE r.project_id = $1
	AND r.id = $2`

	var rule Rule
	err := s.db.GetContext(ctx, &rule, query, projectID, ruleID)
	if err != nil {
		return nil, err
	}

	return &rule, nil
}

type RuleUpdate struct {
	Rule                       *store.JSONB[rules.RuleSet]
	DependsOnEvents            bool
	DependsOnUsers             bool
	DependsOnOrganizations     bool
	DependsOnOrganizationUsers bool
	DependsOnTime              bool
	RecompileInterval          *string
}

func (s *RulesStore) UpdateRule(ctx context.Context, projectID, id uuid.UUID, update RuleUpdate) error {
	query := `
	UPDATE rules
	SET
		rule = COALESCE($3, rule),
		depends_on_events = $4,
		depends_on_users = $5,
		depends_on_organizations = $6,
		depends_on_organization_users = $7,
		depends_on_time = $8,
		recompile_interval = $9::interval
	WHERE project_id = $1
	AND id = $2`

	_, err := s.db.ExecContext(ctx, query, projectID, id, update.Rule, update.DependsOnEvents, update.DependsOnUsers, update.DependsOnOrganizations, update.DependsOnOrganizationUsers, update.DependsOnTime, update.RecompileInterval)
	return err
}

// DuplicateRule creates a copy of an existing rule (with a new ID) within the
// same project. The duplicate inherits the rule definition, dependency flags,
// and event dependencies of the original.
func (s *RulesStore) DuplicateRule(ctx context.Context, projectID, ruleID uuid.UUID) (uuid.UUID, error) {
	stmt := `
	WITH src AS (
		SELECT project_id, rule, depends_on_events, depends_on_users,
			depends_on_organizations, depends_on_organization_users,
			depends_on_time, recompile_interval
		FROM rules
		WHERE project_id = $1 AND id = $2
	),
	new_rule AS (
		INSERT INTO rules (project_id, rule, depends_on_events, depends_on_users,
			depends_on_organizations, depends_on_organization_users,
			depends_on_time, recompile_interval, version)
		SELECT project_id, rule, depends_on_events, depends_on_users,
			depends_on_organizations, depends_on_organization_users,
			depends_on_time, recompile_interval, 1
		FROM src
		RETURNING id
	),
	copy_events AS (
		INSERT INTO rules_events (rule_id, event_id)
		SELECT nr.id, re.event_id
		FROM new_rule nr, rules_events re
		WHERE re.rule_id = $2
	)
	SELECT id FROM new_rule`

	var newID uuid.UUID
	err := s.db.GetContext(ctx, &newID, stmt, projectID, ruleID)
	if err != nil {
		return uuid.Nil, err
	}

	return newID, nil
}

func (s *RulesStore) UnsafeDeleteRule(ctx context.Context, projectID, ruleID uuid.UUID) error {
	query := `
	DELETE FROM rules
	WHERE project_id = $1
	AND id = $2`

	_, err := s.db.ExecContext(ctx, query, projectID, ruleID)
	return err
}
