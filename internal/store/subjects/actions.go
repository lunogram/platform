package subjects

import (
	"context"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/store"
)

type ActionSchemaPath struct {
	Path  string         `db:"path"`
	Types pq.StringArray `db:"types"`
}

func NewActionsStore(db store.DB) *ActionsStore {
	return &ActionsStore{db: db}
}

type ActionsStore struct {
	db store.DB
}

func (s *ActionsStore) UpsertActionSchema(ctx context.Context, actionID uuid.UUID, functionID string, paths rules.Paths) error {
	stmt := `
	INSERT INTO action_schemas (action_id, function_id, path, data_type)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (action_id, function_id, path, data_type) DO NOTHING`

	for _, path := range paths {
		_, err := s.db.ExecContext(ctx, stmt, actionID, functionID, path.Path, path.Type)
		if err != nil {
			return err
		}
	}

	return nil
}

type ActionSchema struct {
	ActionID uuid.UUID `db:"action_id"`
	Schema   []ActionSchemaPath
}

func (s *ActionsStore) ListActionSchemas(ctx context.Context, actionID uuid.UUID, functionID string) ([]ActionSchemaPath, error) {
	stmt := `
	SELECT
		as2.path,
		COALESCE(array_agg(DISTINCT as2.data_type ORDER BY as2.data_type) FILTER (WHERE as2.data_type IS NOT NULL), '{}') as types
	FROM action_schemas as2
	WHERE as2.action_id = $1 AND as2.function_id = $2
	GROUP BY as2.path
	ORDER BY as2.path`

	var rows []ActionSchemaPath
	err := s.db.SelectContext(ctx, &rows, stmt, actionID, functionID)
	if err != nil {
		return nil, err
	}

	return rows, nil
}
