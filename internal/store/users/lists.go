package users

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/rules/query"
	"github.com/lunogram/platform/internal/store"
)

type ListType string

const (
	ListTypeStatic  = "static"
	ListTypeDynamic = "dynamic"
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
	ID         uuid.UUID                   `db:"id"`
	ProjectID  uuid.UUID                   `db:"project_id"`
	Name       string                      `db:"name"`
	Type       ListType                    `db:"type"`
	RuleID     *uuid.UUID                  `db:"rule_id"`
	Rule       *store.JSONB[rules.RuleSet] `db:"rule"`
	Version    int                         `db:"version"`
	UsersCount int                         `db:"users_count"`
	CreatedAt  time.Time                   `db:"created_at"`
	UpdatedAt  time.Time                   `db:"updated_at"`
}

func (list List) OAPI() oapi.List {
	result := oapi.List{
		Id:         list.ID,
		ProjectId:  list.ProjectID,
		Name:       list.Name,
		Type:       oapi.ListType(list.Type),
		UsersCount: list.UsersCount,
		Version:    list.Version,
		CreatedAt:  list.CreatedAt,
		UpdatedAt:  list.UpdatedAt,
	}

	if list.RuleID != nil {
		ruleID := *list.RuleID
		result.RuleId = &ruleID
	}

	if list.Rule != nil {
		result.Rule = &list.Rule.Data
	}

	return result
}

func NewListsStore(db store.DB) *ListsStore {
	return &ListsStore{
		db:    db,
		rules: NewRulesStore(db),
	}
}

type ListsStore struct {
	db    store.DB
	rules *RulesStore
}

func (s *ListsStore) CreateList(ctx context.Context, list List) (uuid.UUID, error) {
	stmt := `
	INSERT INTO lists (project_id, name, type, rule_id, version)
	VALUES ($1, $2, $3, $4, $5)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt, list.ProjectID, list.Name, list.Type, list.RuleID, list.Version)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *ListsStore) ListLists(ctx context.Context, projectID uuid.UUID, pagination store.Pagination) (Lists, int, error) {
	query := `
	SELECT
		l.id,
		l.project_id,
		l.name,
		l.type,
		l.rule_id,
		r.rule,
		l.version,
		COUNT(lu.user_id) AS users_count,
		l.created_at,
		l.updated_at,
		COUNT(*) OVER () AS total_count
	FROM lists l
	LEFT JOIN rules r ON l.rule_id = r.id
	LEFT JOIN list_users lu ON lu.list_id = l.id
	WHERE l.project_id = $1
	AND l.deleted_at IS NULL
	GROUP BY l.id, l.project_id, l.name, l.type, l.rule_id, r.rule, l.version, l.created_at, l.updated_at
	ORDER BY l.updated_at DESC
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
	SELECT
		l.id,
		l.project_id,
		l.name,
		l.type,
		l.rule_id,
		r.rule,
		l.version,
		COALESCE(lu.user_count, 0)::int AS users_count,
		l.created_at,
		l.updated_at
	FROM lists l
	LEFT JOIN rules r ON l.rule_id = r.id
	LEFT JOIN (
		SELECT list_id, COUNT(*) AS user_count
		FROM list_users
		GROUP BY list_id
	) lu ON lu.list_id = l.id
	WHERE l.project_id = $1
	AND l.id = $2
	AND l.deleted_at IS NULL`

	var list List
	err := s.db.GetContext(ctx, &list, query, projectID, listID)
	if err != nil {
		return nil, err
	}

	return &list, nil
}

type ListUpdate struct {
	Name   *string
	RuleID *uuid.UUID
}

func (s *ListsStore) UpdateList(ctx context.Context, projectID, listID uuid.UUID, update ListUpdate) error {
	stmt := `
	UPDATE lists
	SET
		name    = COALESCE($3, name),
		rule_id = COALESCE($4, rule_id)
	WHERE project_id = $1
		AND id = $2
		AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, projectID, listID, update.Name, update.RuleID)
	return err
}

func (s *ListsStore) AddUserToList(ctx context.Context, listID, userID uuid.UUID) error {
	stmt := `
	INSERT INTO list_users (user_id, list_id)
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
	query := `
	INSERT INTO lists (project_id, name, type, rule_id, version)
	SELECT project_id, $1, type, rule_id, 0
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

func (s *ListsStore) SelectListUsers(ctx context.Context, projectID, listID uuid.UUID, pagination store.Pagination) (Users, int, error) {
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
	INNER JOIN list_users ul ON u.id = ul.user_id
	INNER JOIN lists l ON ul.list_id = l.id
	WHERE l.project_id = $1
	AND l.id = $2
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

func (s *ListsStore) SelectListUsersDependency(ctx context.Context, projectID uuid.UUID) ([]uuid.UUID, error) {
	query := `
	SELECT l.id
	FROM lists l
	JOIN rules r ON l.rule_id = r.id
	WHERE l.project_id = $1
	AND r.depends_on_users = TRUE
	AND l.deleted_at IS NULL`

	var result []uuid.UUID
	err := s.db.SelectContext(ctx, &result, query, projectID)
	if err != nil {
		return nil, err
	}

	return result, nil
}

type RecomputeAction string

const (
	RecomputeActionInserted RecomputeAction = "inserted"
	RecomputeActionDeleted  RecomputeAction = "deleted"
)

type Recomputed struct {
	UserID uuid.UUID       `db:"user_id"`
	Action RecomputeAction `db:"action"`
}

// RecomputeList evaluates the given ruleset for the specified project and list
// and updates list membership in the database to match the current result.
//
// The ruleset is compiled into a SQL query which yields the set of user IDs
// that currently qualify for the list ("recomputed"). A single MERGE statement
// is then executed to:
//   - insert a new active list_users row for users that now qualify but are not
//     currently active members of the list
//   - soft-delete (set deleted_at) active list_users rows whose users no longer
//     qualify for the list
//
// Soft-deleted rows are intentionally NOT reactivated. If a user later qualifies
// for the list again, a new list_users row will be inserted so historical
// membership periods remain preserved.
//
// This function only changes database state and does not persist or cache the
// recomputed result. All updates occur in a single SQL statement.
func (s *ListsStore) RecomputeList(ctx context.Context, projectID, listID uuid.UUID, ruleset rules.RuleSet) ([]Recomputed, error) {
	builder := query.NewQueryBuilder(projectID, nil)
	query, err := builder.Query(ruleset)
	if err != nil {
		return nil, err
	}

	args := append(query.Args, listID)
	listIdx := len(args)

	sql := fmt.Sprintf(`
	WITH recomputed AS MATERIALIZED (
		%s
	),
	applied AS (
		MERGE INTO list_users AS lu
		USING recomputed AS rc
		CROSS JOIN (SELECT $%d::uuid AS list_id) AS p
		ON (
			lu.list_id = p.list_id
			AND lu.user_id = rc.id
		)

		WHEN NOT MATCHED THEN
		INSERT (list_id, user_id)
		VALUES (p.list_id, rc.id)

		WHEN NOT MATCHED BY SOURCE AND lu.list_id = p.list_id
		THEN DELETE

		RETURNING
			lu.user_id,
			CASE
				WHEN rc.id IS NULL THEN 'deleted'
				ELSE 'inserted'
			END AS action
	)
	SELECT * FROM applied`, query.SQL, listIdx)

	var results []Recomputed
	err = s.db.SelectContext(ctx, &results, sql, args...)
	if err != nil {
		return results, err
	}

	return results, nil
}
