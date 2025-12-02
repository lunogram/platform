package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/services/nexus/oapi"
)

type ListType string

const (
	ListTypeStatic  = "static"
	ListTypeDynamic = "dynamic"
)

type ListState string

const (
	ListStateDraft = "draft"
	ListStateReady = "ready"
)

type Lists []List

func (lists Lists) OAPI() []oapi.List {
	result := make([]oapi.List, len(lists))
	for index, list := range lists {
		result[index] = list.OAPI()
	}
	return result
}

type List struct {
	ID          uuid.UUID       `db:"id"`
	ProjectID   uuid.UUID       `db:"project_id"`
	Name        string          `db:"name"`
	Type        ListType        `db:"type"`
	State       ListState       `db:"state"`
	Rule        JSONB[RuleData] `db:"rule"`
	RuleID      *uuid.UUID      `db:"rule_id"`
	Version     int             `db:"version"`
	UsersCount  int             `db:"users_count"`
	RefreshedAt *time.Time      `db:"refreshed_at"`
	CreatedAt   time.Time       `db:"created_at"`
	UpdatedAt   time.Time       `db:"updated_at"`
}

type RuleData map[string]any

func (list List) OAPI() oapi.List {
	var ruleRaw json.RawMessage
	if list.Rule.Data != nil {
		ruleRaw, _ = json.Marshal(list.Rule.Data)
	}

	result := oapi.List{
		Id:         list.ID,
		ProjectId:  list.ProjectID,
		Name:       list.Name,
		Type:       oapi.ListType(list.Type),
		State:      oapi.ListState(list.State),
		UsersCount: list.UsersCount,
		Version:    list.Version,
		CreatedAt:  list.CreatedAt,
		UpdatedAt:  list.UpdatedAt,
	}

	if ruleRaw != nil {
		result.Rule = &ruleRaw
	}

	if list.RuleID != nil {
		ruleID := *list.RuleID
		result.RuleId = &ruleID
	}

	if list.RefreshedAt != nil {
		refreshedAt := *list.RefreshedAt
		result.RefreshedAt = &refreshedAt
	}

	return result
}

func NewListsStore(db DB) *ListsStore {
	return &ListsStore{
		db: db,
	}
}

type ListsStore struct {
	db DB
}

func (s *ListsStore) CreateList(ctx context.Context, list List) (uuid.UUID, error) {
	stmt := `
	INSERT INTO lists (project_id, name, type, state, rule, users_count, version)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt, list.ProjectID, list.Name, list.Type, list.State, list.Rule, list.UsersCount, list.Version)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *ListsStore) ListLists(ctx context.Context, projectID uuid.UUID, pagination Pagination) (Lists, int, error) {
	query := `
	SELECT id, project_id, name, type, state, rule, rule_id, version, users_count, refreshed_at, created_at, updated_at,
		COUNT(*) OVER () AS total_count
	FROM lists
	WHERE project_id = $1
	AND deleted_at IS NULL
	ORDER BY updated_at DESC
	LIMIT $2 OFFSET $3`

	var results []struct {
		List
		TotalCount int `db:"total_count"`
	}
	err := s.db.SelectContext(ctx, &results, query, projectID, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	lists := make(Lists, len(results))
	total := 0

	for i, r := range results {
		lists[i] = r.List
		if i == 0 {
			total = r.TotalCount
		}
	}

	return lists, total, nil
}

func (s *ListsStore) GetList(ctx context.Context, projectID, listID uuid.UUID) (*List, error) {
	query := `
	SELECT id, project_id, name, type, state, rule, rule_id, version, users_count, refreshed_at, created_at, updated_at
	FROM lists
	WHERE project_id = $1
	AND id = $2
	AND deleted_at IS NULL`

	var list List
	err := s.db.GetContext(ctx, &list, query, projectID, listID)
	if err != nil {
		return nil, err
	}

	return &list, nil
}

type ListUpdate struct {
	Name      *string
	Rule      *JSONB[RuleData]
	Published *bool
}

func (s *ListsStore) UpdateList(ctx context.Context, projectID, listID uuid.UUID, update ListUpdate) error {
	query := `
	UPDATE lists
	SET
		name = COALESCE($1, name),
		rule = COALESCE($2, rule),
		state = CASE 
			WHEN state = 'draft' AND $3 = true THEN 'ready'
			WHEN state = 'draft' AND $3 = false THEN 'draft'
			ELSE state
		END
	WHERE project_id = $4
	AND id = $5
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, query, update.Name, update.Rule, update.Published, projectID, listID)
	return err
}

func (s *ListsStore) AddUserToList(ctx context.Context, listID, userID uuid.UUID) error {
	stmt := `
	INSERT INTO user_list (user_id, list_id)
	VALUES ($1, $2)
	ON CONFLICT (user_id, list_id) DO NOTHING`

	_, err := s.db.ExecContext(ctx, stmt, userID, listID)
	return err
}

func (s *ListsStore) DeleteList(ctx context.Context, projectID, listID uuid.UUID) error {
	query := `
	UPDATE lists
	SET deleted_at = NOW()
	WHERE project_id = $1
	AND id = $2
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, query, projectID, listID)
	return err
}

func (s *ListsStore) DuplicateList(ctx context.Context, projectID, listID uuid.UUID, newName string) (uuid.UUID, error) {
	// When duplicating a list, version and users_count are reset to 0 to initialize the new list.
	// The duplicated list starts in 'draft' state regardless of the source list's state.
	query := `
	INSERT INTO lists (project_id, name, type, state, rule, rule_id, version, users_count)
	SELECT project_id, $1, type, 'draft', rule, rule_id, 0, 0
	FROM lists
	WHERE project_id = $2
	AND id = $3
	AND deleted_at IS NULL
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, query, newName, projectID, listID)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *ListsStore) ListListUsers(ctx context.Context, projectID, listID uuid.UUID, pagination Pagination) (Users, int, error) {
	query := `
	SELECT 
		u.id, u.project_id, u.anonymous_id, u.external_id, u.email, u.phone, u.data, u.timezone, u.locale, u.version, u.created_at, u.updated_at,
		EXISTS(
			SELECT 1 FROM devices d 
			WHERE d.user_id = u.id
			AND d.token IS NOT NULL 
			AND d.token != ''
		) as has_push_device,
		COUNT(*) OVER () AS total_count
	FROM users u
	INNER JOIN user_list ul ON u.id = ul.user_id
	INNER JOIN lists l ON ul.list_id = l.id
	WHERE l.project_id = $1
	AND l.id = $2
	AND ul.deleted_at IS NULL
	ORDER BY ul.created_at DESC
	LIMIT $3 OFFSET $4`

	type result struct {
		User
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, projectID, listID, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []User{}, 0, nil
	}

	total := results[0].TotalCount
	users := make([]User, len(results))

	for index, result := range results {
		users[index] = result.User
	}

	return users, total, nil
}
