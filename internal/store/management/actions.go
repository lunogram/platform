package management

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store"
)

type Actions []Action

func (a Actions) OAPI() []oapi.Action {
	results := make([]oapi.Action, len(a))
	for i, action := range a {
		results[i] = action.OAPI()
	}
	return results
}

type Action struct {
	ID        uuid.UUID        `db:"id"`
	ProjectID uuid.UUID        `db:"project_id"`
	Name      string           `db:"name"`
	Type      string           `db:"type"`
	Config    store.JSONB[any] `db:"config"`
	CreatedAt time.Time        `db:"created_at"`
	UpdatedAt time.Time        `db:"updated_at"`
}

func (a *Action) OAPI() oapi.Action {
	config := json.RawMessage("{}")
	if a.Config.Data != nil {
		if b, err := json.Marshal(a.Config.Data); err == nil {
			config = b
		}
	}

	return oapi.Action{
		Id:        a.ID,
		ProjectId: a.ProjectID,
		Name:      a.Name,
		Type:      oapi.ActionType(a.Type),
		Config:    config,
		CreatedAt: a.CreatedAt,
		UpdatedAt: a.UpdatedAt,
	}
}

func NewActionsStore(db store.DB) *ActionsStore {
	return &ActionsStore{db: db}
}

type ActionsStore struct {
	db store.DB
}

func (s *ActionsStore) CreateAction(ctx context.Context, projectID uuid.UUID, name string, actionType string, config json.RawMessage) (*Action, error) {
	if config == nil {
		config = json.RawMessage("{}")
	}

	stmt := `
	INSERT INTO actions (project_id, name, type, config)
	VALUES ($1, $2, $3, $4)
	RETURNING id, project_id, name, type, config, created_at, updated_at`

	var action Action
	err := s.db.GetContext(ctx, &action, stmt, projectID, name, actionType, config)
	if err != nil {
		return nil, err
	}

	return &action, nil
}

func (s *ActionsStore) GetAction(ctx context.Context, projectID, actionID uuid.UUID) (*Action, error) {
	stmt := `
	SELECT id, project_id, name, type, config, created_at, updated_at
	FROM actions
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL`

	var action Action
	err := s.db.GetContext(ctx, &action, stmt, actionID, projectID)
	if err != nil {
		return nil, err
	}

	return &action, nil
}

func (s *ActionsStore) ListActions(ctx context.Context, projectID uuid.UUID, pagination store.Pagination, search string) (Actions, int, error) {
	query := `
	SELECT
		id,
		project_id,
		name,
		type,
		config,
		created_at,
		updated_at,
		COUNT(*) OVER () AS total_count
	FROM actions
	WHERE project_id = $1 AND deleted_at IS NULL
		AND ($4 = '' OR name ILIKE '%' || $4 || '%')
	ORDER BY created_at DESC
	LIMIT $2 OFFSET $3`

	type result struct {
		Action
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, projectID, pagination.Limit, pagination.Offset, search)
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []Action{}, 0, nil
	}

	total := results[0].TotalCount
	actions := make([]Action, len(results))

	for index, r := range results {
		actions[index] = r.Action
	}

	return actions, total, nil
}

func (s *ActionsStore) UpdateAction(ctx context.Context, projectID, actionID uuid.UUID, name *string, actionType *string, config json.RawMessage) error {
	stmt := `
	UPDATE actions
	SET
		name = COALESCE($1, name),
		type = COALESCE($2, type),
		config = COALESCE($3, config)
	WHERE id = $4 AND project_id = $5 AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, name, actionType, config, actionID, projectID)
	return err
}

func (s *ActionsStore) DeleteAction(ctx context.Context, projectID, actionID uuid.UUID) error {
	stmt := `
	UPDATE actions
	SET deleted_at = NOW()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, actionID, projectID)
	return err
}
