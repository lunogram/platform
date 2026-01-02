package store

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/lunogram/platform/services/nexus/internal/rules"
)

type EventSchemaPath struct {
	Path  string         `db:"path"`
	Types pq.StringArray `db:"types"`
}

type Event struct {
	ID     uuid.UUID `db:"id"`
	Name   string    `db:"name"`
	Schema []EventSchemaPath
}

type JourneyEntranceStep struct {
	JourneyID  uuid.UUID                  `db:"journey_id"`
	VersionID  uuid.UUID                  `db:"version_id"`
	StepID     uuid.UUID                  `db:"step_id"`
	ExternalID string                     `db:"external_id"`
	Type       string                     `db:"type"`
	DataKey    *string                    `db:"data_key"`
	Data       *json.RawMessage           `db:"data"`
	Children   JourneyVersionStepChildren `db:"children"`
}

func NewEventsStore(db DB) *EventsStore {
	return &EventsStore{db: db}
}

type EventsStore struct {
	db DB
}

func (s *EventsStore) UpsertEvent(ctx context.Context, projectID uuid.UUID, name string) (uuid.UUID, error) {
	// NOTE: we have to execute DO UPDATE SET to ensure the id is returned in case of conflict
	stmt := `
	INSERT INTO events (project_id, name)
	VALUES ($1, $2)
	ON CONFLICT (project_id, name)
	DO UPDATE SET name = EXCLUDED.name, deleted_at = NULL
	RETURNING id`

	var eventID uuid.UUID
	err := s.db.GetContext(ctx, &eventID, stmt, projectID, name)
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
	ID    uuid.UUID      `db:"id"`
	Name  string         `db:"name"`
	Path  *string        `db:"path"`
	Types pq.StringArray `db:"types"`
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
				ID:     row.ID,
				Name:   row.Name,
				Schema: []EventSchemaPath{},
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

func (s *EventsStore) ListEvents(ctx context.Context, projectID uuid.UUID) ([]Event, error) {
	stmt := `
	SELECT 
		e.id,
		e.name,
		es.path,
		COALESCE(array_agg(DISTINCT es.data_type ORDER BY es.data_type) FILTER (WHERE es.data_type IS NOT NULL), '{}') as types
	FROM events e
	LEFT JOIN event_schemas es ON e.id = es.event_id
	WHERE e.project_id = $1
	GROUP BY e.id, e.name, es.path
	ORDER BY e.name, es.path`

	var rows eventSchemaRows
	err := s.db.SelectContext(ctx, &rows, stmt, projectID)
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

func (s *EventsStore) ListEventJourneyDependencies(ctx context.Context, eventID uuid.UUID) ([]JourneyEntranceStep, error) {
	query := `
	SELECT
		j.id AS journey_id,
		jv.id AS version_id,
		jvs.id AS step_id,
		jvs.external_id,
		jvs.type,
		jvs.data_key,
		jvs.data,
		COALESCE(
			json_agg(row_to_json(c)) FILTER (WHERE c.version_id IS NOT NULL),
			'[]'
		) AS children
	FROM journeys j
	JOIN journey_versions jv ON jv.id = j.version_id AND jv.status = 'published'
	JOIN journey_version_step_events jvse ON jvse.version_id = jv.id
	JOIN journey_version_steps jvs ON jvs.version_id = jv.id AND jvs.external_id = jvse.external_id
	LEFT JOIN journey_version_step_children c ON jvs.version_id = c.version_id AND jvs.external_id = c.parent_external_id
	WHERE jvse.event_id = $1
	AND j.deleted_at IS NULL
	AND jvs.type = 'entrance'
	GROUP BY j.id, jv.id, jvs.id`

	var entrances []JourneyEntranceStep
	err := s.db.SelectContext(ctx, &entrances, query, eventID)
	return entrances, err
}
