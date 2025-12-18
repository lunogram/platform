package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/lunogram/platform/services/nexus/internal/rules"
)

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
	DO UPDATE SET name = EXCLUDED.name
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

type Event struct {
	ID    uuid.UUID      `db:"id"`
	Name  string         `db:"name"`
	Paths pq.StringArray `db:"paths"`
	Types pq.StringArray `db:"types"`
}

func (s *EventsStore) ListEvents(ctx context.Context, projectID uuid.UUID) ([]Event, error) {
	stmt := `
	SELECT 
		e.id,
		e.name,
		COALESCE(array_agg(es.path ORDER BY es.path) FILTER (WHERE es.path IS NOT NULL), '{}') as paths,
		COALESCE(array_agg(es.data_type ORDER BY es.path) FILTER (WHERE es.data_type IS NOT NULL), '{}') as types
	FROM events e
	LEFT JOIN event_schemas es ON e.id = es.event_id
	WHERE e.project_id = $1
	GROUP BY e.id, e.name
	ORDER BY e.name`

	var events []Event
	err := s.db.SelectContext(ctx, &events, stmt, projectID)
	if err != nil {
		return nil, err
	}

	return events, nil
}
