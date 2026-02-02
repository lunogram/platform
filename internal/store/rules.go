package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rules"
)

type Rule struct {
	ID              uuid.UUID            `db:"id"`
	ProjectID       uuid.UUID            `db:"project_id"`
	Rule            JSONB[rules.RuleSet] `db:"rule"`
	DependsOnEvents bool                 `db:"depends_on_events"`
	DependsOnUsers  bool                 `db:"depends_on_users"`
	Events          []uuid.UUID          `db:"events"`
	Version         int                  `db:"version"`
	CreatedAt       time.Time            `db:"created_at"`
	UpdatedAt       time.Time            `db:"updated_at"`
}

func NewRulesStore(db DB) *RulesStore {
	return &RulesStore{
		db: db,
	}
}

type RulesStore struct {
	db DB
}

func (s *RulesStore) CreateOrUpdateRule(ctx context.Context, projectID uuid.UUID, id *uuid.UUID, rule rules.RuleSet) (uuid.UUID, error) {
	if id != nil {
		err := s.UpdateRule(ctx, projectID, *id, RuleUpdate{
			Rule:            &JSONB[rules.RuleSet]{Data: rule},
			DependsOnEvents: rule.DependsOnEvents(),
			DependsOnUsers:  rule.DependsOnUsers(),
		})

		return *id, err
	}

	return s.CreateRule(ctx, Rule{
		ProjectID:       projectID,
		Rule:            JSONB[rules.RuleSet]{Data: rule},
		DependsOnEvents: rule.DependsOnEvents(),
		DependsOnUsers:  rule.DependsOnUsers(),
		Version:         1,
	})
}

func (s *RulesStore) CreateRule(ctx context.Context, rule Rule) (uuid.UUID, error) {
	stmt := `
	INSERT INTO rules (project_id, rule, depends_on_events, depends_on_users, version)
	VALUES ($1, $2, $3, $4, $5)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt, rule.ProjectID, rule.Rule, rule.DependsOnEvents, rule.DependsOnUsers, rule.Version)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *RulesStore) SetRuleEventDependencies(ctx context.Context, projectID, ruleID uuid.UUID, events []string) error {
	query := `
	WITH event_ids AS (
		SELECT e.id
		FROM events e
		WHERE e.project_id = $1
		AND e.name = ANY($3)
		AND e.deleted_at IS NULL
	),
	deleted AS (
		DELETE FROM rules_events
		WHERE rule_id = $2
		AND event_id NOT IN (SELECT id FROM event_ids)
	)
	INSERT INTO rules_events (rule_id, event_id)
	SELECT $2, id FROM event_ids
	ON CONFLICT (rule_id, event_id) DO NOTHING`

	_, err := s.db.ExecContext(ctx, query, projectID, ruleID, events)
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
	Rule            *JSONB[rules.RuleSet]
	DependsOnEvents bool
	DependsOnUsers  bool
}

func (s *RulesStore) UpdateRule(ctx context.Context, projectID, id uuid.UUID, update RuleUpdate) error {
	query := `
	UPDATE rules
	SET
		rule = COALESCE($3, rule),
		depends_on_events = $4,
		depends_on_users = $5
	WHERE project_id = $1
	AND id = $2`

	_, err := s.db.ExecContext(ctx, query, projectID, id, update.Rule, update.DependsOnEvents, update.DependsOnUsers)
	return err
}

func (s *RulesStore) UnsafeDeleteRule(ctx context.Context, projectID, ruleID uuid.UUID) error {
	query := `
	DELETE FROM rules
	WHERE project_id = $1
	AND id = $2`

	_, err := s.db.ExecContext(ctx, query, projectID, ruleID)
	return err
}
