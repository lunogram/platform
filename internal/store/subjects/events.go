package subjects

import (
	"context"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/store"
)

// SubjectType represents the type of subject for events and schemas
type SubjectType string

const (
	SubjectTypeUser             SubjectType = "user"
	SubjectTypeOrganization     SubjectType = "organization"
	SubjectTypeOrganizationUser SubjectType = "organization_user"
)

type EventSchemaPath struct {
	Path  string         `db:"path"`
	Types pq.StringArray `db:"types"`
}

type Event struct {
	ID          uuid.UUID   `db:"id"`
	Name        string      `db:"name"`
	SubjectType SubjectType `db:"subject_type"`
	Schema      []EventSchemaPath
}

func NewEventsStore(db store.DB) *EventsStore {
	return &EventsStore{db: db}
}

type EventsStore struct {
	db store.DB
}

func (s *EventsStore) UpsertEvent(ctx context.Context, projectID uuid.UUID, name string, subjectType SubjectType) (uuid.UUID, error) {
	// NOTE: we have to execute DO UPDATE SET to ensure the id is returned in case of conflict
	stmt := `
	INSERT INTO events (project_id, name, subject_type)
	VALUES ($1, $2, $3)
	ON CONFLICT (project_id, name, subject_type)
	DO UPDATE SET name = EXCLUDED.name, deleted_at = NULL
	RETURNING id`

	var eventID uuid.UUID
	err := s.db.GetContext(ctx, &eventID, stmt, projectID, name, subjectType)
	if err != nil {
		return uuid.Nil, err
	}

	return eventID, nil
}

func (s *EventsStore) UpsertEventSchema(ctx context.Context, projectID, eventID uuid.UUID, paths rules.Paths) error {
	stmt := `
	INSERT INTO event_schemas (event_id, path, data_type)
	VALUES ($1, $2, $3)
	ON CONFLICT (event_id, path, data_type) DO NOTHING`

	// TODO: optimize with batch insert
	for _, path := range paths {
		_, err := s.db.ExecContext(ctx, stmt, eventID, path.Path, path.Type)
		if err != nil {
			return err
		}
	}

	return nil
}

type eventSchemaRow struct {
	ID          uuid.UUID      `db:"id"`
	Name        string         `db:"name"`
	SubjectType SubjectType    `db:"subject_type"`
	Path        *string        `db:"path"`
	Types       pq.StringArray `db:"types"`
}

type eventSchemaRows []eventSchemaRow

func (rows eventSchemaRows) ToEvents() []Event {
	lookup := make(map[uuid.UUID]int)
	results := make([]Event, 0)

	for _, row := range rows {
		index, has := lookup[row.ID]
		if !has {
			lookup[row.ID] = len(results)
			results = append(results, Event{
				ID:          row.ID,
				Name:        row.Name,
				SubjectType: row.SubjectType,
				Schema:      []EventSchemaPath{},
			})
			index = lookup[row.ID]
		}

		if row.Path != nil {
			results[index].Schema = append(results[index].Schema, EventSchemaPath{
				Path:  *row.Path,
				Types: row.Types,
			})
		}
	}

	return results
}

func (s *EventsStore) ListEventSchemas(ctx context.Context, projectID uuid.UUID, subjectType SubjectType) ([]Event, error) {
	stmt := `
	SELECT
		e.id,
		e.name,
		e.subject_type,
		es.path,
		COALESCE(array_agg(DISTINCT es.data_type ORDER BY es.data_type) FILTER (WHERE es.data_type IS NOT NULL), '{}') as types
	FROM events e
	LEFT JOIN event_schemas es ON e.id = es.event_id
	WHERE e.project_id = $1 AND e.subject_type = $2
	GROUP BY e.id, e.name, e.subject_type, es.path
	ORDER BY e.name, es.path`

	var rows eventSchemaRows
	err := s.db.SelectContext(ctx, &rows, stmt, projectID, subjectType)
	if err != nil {
		return nil, err
	}

	return rows.ToEvents(), nil
}

func (s *EventsStore) ListEventListDependencies(ctx context.Context, id uuid.UUID) ([]uuid.UUID, error) {
	stmt := `
	SELECT l.id AS list_id
	FROM rules_events re
	JOIN lists l ON l.rule_id = re.rule_id
	WHERE re.event_id = $1
	AND l.deleted_at IS NULL`

	var ids []uuid.UUID
	err := s.db.SelectContext(ctx, &ids, stmt, id)
	if err != nil {
		return nil, err
	}

	return ids, nil
}
