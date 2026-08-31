package subjects

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/node/metrics"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/rules/query"
	"github.com/lunogram/platform/internal/store"
)

type ListType string

const (
	ListTypeStatic  = "static"
	ListTypeDynamic = "dynamic"
)

type ListVersionStatus string

const (
	ListVersionStatusDraft     ListVersionStatus = "draft"
	ListVersionStatusPublished ListVersionStatus = "published"
	ListVersionStatusArchived  ListVersionStatus = "archived"
)

type Lists []List

func (lists Lists) OAPI() []oapi.List {
	result := make([]oapi.List, len(lists))
	for index, list := range lists {
		result[index] = list.OAPI()
	}
	return result
}

// ListVersion represents a single version record for a list.
type ListVersion struct {
	ID            uuid.UUID         `db:"id"`
	ListID        uuid.UUID         `db:"list_id"`
	VersionNumber int               `db:"version_number"`
	Status        ListVersionStatus `db:"status"`
	RuleID        *uuid.UUID        `db:"rule_id"`
	CreatedAt     time.Time         `db:"created_at"`
	PublishedAt   *time.Time        `db:"published_at"`
}

type List struct {
	ID            uuid.UUID                   `db:"id"`
	ProjectID     uuid.UUID                   `db:"project_id"`
	Name          string                      `db:"name"`
	Type          ListType                    `db:"type"`
	VersionID     *uuid.UUID                  `db:"version_id"`
	State         ListVersionStatus           `db:"state"`
	Rule          *store.JSONB[rules.RuleSet] `db:"rule"`
	DraftRule     *store.JSONB[rules.RuleSet] `db:"draft_rule"`
	VersionNumber int                         `db:"version_number"`
	Version       int                         `db:"version"`
	UsersCount    int                         `db:"users_count"`
	CreatedAt     time.Time                   `db:"created_at"`
	UpdatedAt     time.Time                   `db:"updated_at"`
	DeletedAt     *time.Time                  `db:"deleted_at"`
}

func (list List) OAPI() oapi.List {
	// Map DB version status to OAPI state: published -> ready, draft stays draft
	state := oapi.ListStateDraft
	switch list.State {
	case ListVersionStatusPublished:
		state = oapi.ListStateReady
	case ListVersionStatusDraft:
		state = oapi.ListStateDraft
	}

	archived := list.DeletedAt != nil
	result := oapi.List{
		Id:         list.ID,
		ProjectId:  list.ProjectID,
		Name:       list.Name,
		Type:       oapi.ListType(list.Type),
		State:      state,
		UsersCount: list.UsersCount,
		Version:    list.Version,
		CreatedAt:  list.CreatedAt,
		UpdatedAt:  list.UpdatedAt,
		Archived:   &archived,
	}

	if list.VersionNumber > 0 {
		vn := list.VersionNumber
		result.VersionNumber = &vn
	}

	if list.Rule != nil {
		result.Rule = &list.Rule.Data
	}

	if list.DraftRule != nil {
		result.DraftRule = &list.DraftRule.Data
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

func (s *ListsStore) CountLists(ctx context.Context, projectID uuid.UUID) (int, error) {
	var count int
	err := s.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM lists WHERE project_id = $1 AND deleted_at IS NULL`, projectID)
	return count, err
}

func (s *ListsStore) CreateList(ctx context.Context, list List) (uuid.UUID, error) {
	stmt := `
	INSERT INTO lists (project_id, name, type, version)
	VALUES ($1, $2, $3, $4)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt, list.ProjectID, list.Name, list.Type, list.Version)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

// listSelectFields returns the common SELECT clause for list queries.
// It joins through list_versions to derive state, published rule, draft rule, and version number.
const listSelectFields = `
	l.id,
	l.project_id,
	l.name,
	l.type,
	l.version_id,
	COALESCE(active_v.status, 'draft') AS state,
	pub_r.rule AS rule,
	draft_r.rule AS draft_rule,
	COALESCE(active_v.version_number, 0) AS version_number,
	l.version,
	l.deleted_at`

// listSelectJoins returns the common JOIN clause for list queries.
// - active_v: the version pointed to by lists.version_id
// - pub_v: the latest published version (for the published rule)
// - pub_r: the rule from the published version
// - draft_v: the draft version (if one exists)
// - draft_r: the rule from the draft version
const listSelectJoins = `
	LEFT JOIN list_versions active_v ON l.version_id = active_v.id
	LEFT JOIN LATERAL (
		SELECT lv.rule_id
		FROM list_versions lv
		WHERE lv.list_id = l.id AND lv.status = 'published'
		ORDER BY lv.version_number DESC
		LIMIT 1
	) pub_v ON TRUE
	LEFT JOIN rules pub_r ON pub_v.rule_id = pub_r.id
	LEFT JOIN list_versions draft_v ON draft_v.list_id = l.id AND draft_v.status = 'draft'
	LEFT JOIN rules draft_r ON draft_v.rule_id = draft_r.id`

func (s *ListsStore) ListLists(ctx context.Context, projectID uuid.UUID, pagination store.Pagination, search string, archivedOnly bool) (Lists, int, error) {
	q := fmt.Sprintf(`
	SELECT
		%s,
		COUNT(lu.user_id) AS users_count,
		l.created_at,
		l.updated_at,
		COUNT(*) OVER () AS total_count
	FROM lists l
	%s
	LEFT JOIN list_users lu ON lu.list_id = l.id
	WHERE l.project_id = $1
	AND (($5 = false AND l.deleted_at IS NULL) OR ($5 = true AND l.deleted_at IS NOT NULL))
	AND ($4 = '' OR l.name ILIKE '%%' || $4 || '%%')
	GROUP BY l.id, l.project_id, l.name, l.type, l.version_id,
		active_v.status, pub_r.rule, draft_r.rule, active_v.version_number,
		l.version, l.created_at, l.updated_at
	ORDER BY l.updated_at DESC
	LIMIT $2 OFFSET $3`, listSelectFields, listSelectJoins)

	var results []struct {
		List
		TotalCount int `db:"total_count"`
	}
	err := s.db.SelectContext(ctx, &results, q, projectID, pagination.Limit, pagination.Offset, search, archivedOnly)
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
	q := fmt.Sprintf(`
	SELECT
		%s,
		COALESCE(lu.user_count, 0)::int AS users_count,
		l.created_at,
		l.updated_at
	FROM lists l
	%s
	LEFT JOIN (
		SELECT list_id, COUNT(*) AS user_count
		FROM list_users
		GROUP BY list_id
	) lu ON lu.list_id = l.id
	WHERE l.project_id = $1
	AND l.id = $2
	AND l.deleted_at IS NULL`, listSelectFields, listSelectJoins)

	var list List
	err := s.db.GetContext(ctx, &list, q, projectID, listID)
	if err != nil {
		return nil, err
	}

	return &list, nil
}

type ListUpdate struct {
	Name *string
}

func (s *ListsStore) UpdateList(ctx context.Context, projectID, listID uuid.UUID, update ListUpdate) error {
	stmt := `
	UPDATE lists
	SET
		name = COALESCE($3, name)
	WHERE project_id = $1
		AND id = $2
		AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, projectID, listID, update.Name)
	return err
}

// CreateVersion creates a new list version record.
func (s *ListsStore) CreateVersion(ctx context.Context, listID uuid.UUID, ruleID *uuid.UUID) (uuid.UUID, error) {
	stmt := `
	INSERT INTO list_versions (list_id, version_number, status, rule_id)
	VALUES ($1, COALESCE((SELECT MAX(version_number) FROM list_versions WHERE list_id = $1), 0) + 1, 'draft', $2)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt, listID, ruleID)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

// SetListVersionID updates the lists.version_id pointer to the given version.
func (s *ListsStore) SetListVersionID(ctx context.Context, listID, versionID uuid.UUID) error {
	stmt := `UPDATE lists SET version_id = $2 WHERE id = $1`
	_, err := s.db.ExecContext(ctx, stmt, listID, versionID)
	return err
}

// EnsureDraftVersion returns the ID of an existing draft version for the list,
// or creates a new one by copying the rule from the current published version.
// It also swings lists.version_id to point to the draft.
func (s *ListsStore) EnsureDraftVersion(ctx context.Context, projectID, listID uuid.UUID) (*ListVersion, error) {
	// Check for existing draft
	var existing ListVersion
	err := s.db.GetContext(ctx, &existing, `
		SELECT id, list_id, version_number, status, rule_id, created_at, published_at
		FROM list_versions
		WHERE list_id = $1 AND status = 'draft'`, listID)
	if err == nil {
		// Draft exists — ensure version_id points to it
		err = s.SetListVersionID(ctx, listID, existing.ID)
		if err != nil {
			return nil, err
		}
		return &existing, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	// No draft exists — get the published version's rule_id to copy
	var publishedRuleID *uuid.UUID
	err = s.db.GetContext(ctx, &publishedRuleID, `
		SELECT rule_id FROM list_versions
		WHERE list_id = $1 AND status = 'published'
		ORDER BY version_number DESC LIMIT 1`, listID)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// If there's a published rule, duplicate it for the draft
	var newRuleID *uuid.UUID
	if publishedRuleID != nil {
		id, err := s.rules.DuplicateRule(ctx, projectID, *publishedRuleID)
		if err != nil {
			return nil, err
		}
		newRuleID = &id
	}

	// Create the new draft version
	versionID, err := s.CreateVersion(ctx, listID, newRuleID)
	if err != nil {
		return nil, err
	}

	// Swing version_id to the new draft
	err = s.SetListVersionID(ctx, listID, versionID)
	if err != nil {
		return nil, err
	}

	// Fetch and return the created version
	var version ListVersion
	err = s.db.GetContext(ctx, &version, `
		SELECT id, list_id, version_number, status, rule_id, created_at, published_at
		FROM list_versions WHERE id = $1`, versionID)
	if err != nil {
		return nil, err
	}

	return &version, nil
}

// GetDraftVersion returns the draft version for a list, if one exists.
func (s *ListsStore) GetDraftVersion(ctx context.Context, listID uuid.UUID) (*ListVersion, error) {
	var version ListVersion
	err := s.db.GetContext(ctx, &version, `
		SELECT id, list_id, version_number, status, rule_id, created_at, published_at
		FROM list_versions
		WHERE list_id = $1 AND status = 'draft'
		ORDER BY version_number DESC LIMIT 1`, listID)
	if err != nil {
		return nil, err
	}
	return &version, nil
}

// GetPublishedVersion returns the published version for a list, if one exists.
func (s *ListsStore) GetPublishedVersion(ctx context.Context, listID uuid.UUID) (*ListVersion, error) {
	var version ListVersion
	err := s.db.GetContext(ctx, &version, `
		SELECT id, list_id, version_number, status, rule_id, created_at, published_at
		FROM list_versions
		WHERE list_id = $1 AND status = 'published'
		ORDER BY version_number DESC LIMIT 1`, listID)
	if err != nil {
		return nil, err
	}
	return &version, nil
}

// UpdateVersionRuleID updates the rule_id on a list version.
func (s *ListsStore) UpdateVersionRuleID(ctx context.Context, versionID, ruleID uuid.UUID) error {
	stmt := `UPDATE list_versions SET rule_id = $2 WHERE id = $1`
	_, err := s.db.ExecContext(ctx, stmt, versionID, ruleID)
	return err
}

// PublishVersion archives any currently published version, promotes the draft
// to published, and swings lists.version_id to the newly published version.
func (s *ListsStore) PublishVersion(ctx context.Context, listID, versionID uuid.UUID) error {
	// Archive currently published version(s)
	_, err := s.db.ExecContext(ctx, `
		UPDATE list_versions SET status = 'archived'
		WHERE list_id = $1 AND status = 'published'`, listID)
	if err != nil {
		return err
	}

	// Promote draft to published
	_, err = s.db.ExecContext(ctx, `
		UPDATE list_versions SET status = 'published', published_at = now()
		WHERE id = $1 AND list_id = $2`, versionID, listID)
	if err != nil {
		return err
	}

	// Swing version_id to the published version
	return s.SetListVersionID(ctx, listID, versionID)
}

func (s *ListsStore) AddUserToList(ctx context.Context, listID, userID uuid.UUID) error {
	_, err := s.AddUserToListReturning(ctx, listID, userID)
	return err
}

// AddUserToListReturning adds a user to a list and reports whether a new
// membership row was actually created (false when the user was already a
// member). Callers use this to emit list.user.added events only for genuine
// additions, keeping static lists consistent with dynamic recompute.
func (s *ListsStore) AddUserToListReturning(ctx context.Context, listID, userID uuid.UUID) (bool, error) {
	stmt := `
	INSERT INTO list_users (user_id, list_id)
	VALUES ($1, $2)
	ON CONFLICT (user_id, list_id) DO NOTHING
	RETURNING user_id`

	var inserted uuid.UUID
	err := s.db.GetContext(ctx, &inserted, stmt, userID, listID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
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

func (s *ListsStore) UnarchiveList(ctx context.Context, projectID, listID uuid.UUID) error {
	query := `
	UPDATE lists
	SET deleted_at = NULL
	WHERE project_id = $1
	AND id = $2
	AND deleted_at IS NOT NULL`

	result, err := s.db.ExecContext(ctx, query, projectID, listID)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *ListsStore) DuplicateList(ctx context.Context, projectID, listID uuid.UUID, newName string) (uuid.UUID, error) {
	// Create the new list (without version_id — it starts as a draft)
	q := `
	INSERT INTO lists (project_id, name, type, version)
	SELECT project_id, $1, type, 0
	FROM lists
	WHERE project_id = $2
	AND id = $3
	AND deleted_at IS NULL
	RETURNING id`

	var newListID uuid.UUID
	err := s.db.GetContext(ctx, &newListID, q, newName, projectID, listID)
	if err != nil {
		return uuid.Nil, err
	}

	// Copy the published version's rule into a draft version for the new list
	var publishedRuleID *uuid.UUID
	err = s.db.GetContext(ctx, &publishedRuleID, `
		SELECT lv.rule_id FROM list_versions lv
		WHERE lv.list_id = $1 AND lv.status = 'published'
		ORDER BY lv.version_number DESC LIMIT 1`, listID)
	if err != nil && err != sql.ErrNoRows {
		return uuid.Nil, err
	}

	if publishedRuleID != nil {
		// Duplicate the rule
		newRuleID, err := s.rules.DuplicateRule(ctx, projectID, *publishedRuleID)
		if err != nil {
			return uuid.Nil, err
		}

		// Create a draft version for the new list
		versionID, err := s.CreateVersion(ctx, newListID, &newRuleID)
		if err != nil {
			return uuid.Nil, err
		}

		err = s.SetListVersionID(ctx, newListID, versionID)
		if err != nil {
			return uuid.Nil, err
		}
	}

	return newListID, nil
}

func (s *ListsStore) SelectListUsers(ctx context.Context, projectID, listID uuid.UUID, pagination store.Pagination, search string) (Users, int, error) {
	query := `
	SELECT
		u.id, u.project_id, u.email, u.phone, u.data, u.timezone, u.locale, u.version, u.created_at, u.updated_at,
		EXISTS(
			SELECT 1 FROM user_devices d
			WHERE d.user_id = u.id
			AND d.config IS NOT NULL
			AND d.deleted_at IS NULL
		) as has_push_device,
		COALESCE(ueia.external_ids, '[]'::jsonb) AS external_ids,
		COUNT(*) OVER () AS total_count
	FROM users u
	INNER JOIN list_users ul ON u.id = ul.user_id
	INNER JOIN lists l ON ul.list_id = l.id
	LEFT JOIN user_external_ids_agg ueia ON ueia.user_id = u.id
	WHERE l.project_id = $1
	AND l.id = $2
	AND (
		$3 = '' OR
		u.email ILIKE '%' || $3 || '%' OR
		u.phone ILIKE '%' || $3 || '%' OR
		EXISTS(
			SELECT 1 FROM user_external_ids uei
			WHERE uei.user_id = u.id
			AND uei.external_id ILIKE '%' || $3 || '%'
		)
	)
	ORDER BY ul.created_at DESC
	LIMIT $4 OFFSET $5`

	type result struct {
		User
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, projectID, listID, search, pagination.Limit, pagination.Offset)
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

// PreviewListUsers runs the given ruleset as a read-only query and returns the
// matching users without modifying the list_users table. This is used to show a
// preview of which users would be included when the draft rule is published.
func (s *ListsStore) PreviewListUsers(ctx context.Context, projectID uuid.UUID, ruleset rules.RuleSet, limit int) (Users, int, error) {
	start := time.Now()

	builder := query.NewQueryBuilder(projectID, nil)
	query, err := builder.Query(ruleset)
	if err != nil {
		metrics.ObserveDatasetQuery(metrics.QueryListPreview, projectID, time.Since(start), 0, err)
		return nil, 0, err
	}

	sql := fmt.Sprintf(`
	WITH matched AS MATERIALIZED (
		%s
	)
	SELECT
		u.id, u.project_id, u.email, u.phone, u.data, u.timezone, u.locale, u.version, u.created_at, u.updated_at,
		EXISTS(
			SELECT 1 FROM user_devices d
			WHERE d.user_id = u.id
			AND d.config IS NOT NULL
			AND d.deleted_at IS NULL
		) as has_push_device,
		COALESCE(ueia.external_ids, '[]'::jsonb) AS external_ids,
		COUNT(*) OVER () AS total_count
	FROM users u
	INNER JOIN matched m ON u.id = m.id
	LEFT JOIN user_external_ids_agg ueia ON ueia.user_id = u.id
	ORDER BY u.created_at DESC
	LIMIT %d`, query.SQL, limit)

	type result struct {
		User
		TotalCount int `db:"total_count"`
	}

	var results []result
	err = s.db.SelectContext(ctx, &results, sql, query.Args...)
	if err != nil {
		metrics.ObserveDatasetQuery(metrics.QueryListPreview, projectID, time.Since(start), 0, err)
		return nil, 0, err
	}

	if len(results) == 0 {
		metrics.ObserveDatasetQuery(metrics.QueryListPreview, projectID, time.Since(start), 0, nil)
		return []User{}, 0, nil
	}

	total := results[0].TotalCount
	// The full match size, not the page: the preview is limited but the
	// underlying scan is not, and the scan is what costs.
	metrics.ObserveDatasetQuery(metrics.QueryListPreview, projectID, time.Since(start), total, nil)
	users := make([]User, len(results))
	for i, r := range results {
		users[i] = r.User
	}

	return users, total, nil
}

// SelectListUsersDependency returns IDs of published lists whose rules depend on user data.
func (s *ListsStore) SelectListUsersDependency(ctx context.Context, projectID uuid.UUID) ([]uuid.UUID, error) {
	query := `
	SELECT l.id
	FROM lists l
	JOIN list_versions lv ON lv.list_id = l.id AND lv.status = 'published'
	JOIN rules r ON lv.rule_id = r.id
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

// SelectListOrganizationsDependency returns IDs of published lists whose rules depend on organization data.
func (s *ListsStore) SelectListOrganizationsDependency(ctx context.Context, projectID uuid.UUID) ([]uuid.UUID, error) {
	query := `
	SELECT l.id
	FROM lists l
	JOIN list_versions lv ON lv.list_id = l.id AND lv.status = 'published'
	JOIN rules r ON lv.rule_id = r.id
	WHERE l.project_id = $1
	AND r.depends_on_organizations = TRUE
	AND l.deleted_at IS NULL`

	var result []uuid.UUID
	err := s.db.SelectContext(ctx, &result, query, projectID)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// SelectListOrganizationUsersDependency returns IDs of published lists whose rules depend on organization user data.
func (s *ListsStore) SelectListOrganizationUsersDependency(ctx context.Context, projectID uuid.UUID) ([]uuid.UUID, error) {
	query := `
	SELECT l.id
	FROM lists l
	JOIN list_versions lv ON lv.list_id = l.id AND lv.status = 'published'
	JOIN rules r ON lv.rule_id = r.id
	WHERE l.project_id = $1
	AND r.depends_on_organization_users = TRUE
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
	start := time.Now()

	builder := query.NewQueryBuilder(projectID, nil)
	query, err := builder.Query(ruleset)
	if err != nil {
		metrics.ObserveDatasetQuery(metrics.QueryListRecompute, projectID, time.Since(start), 0, err)
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
	// Rows here are membership *changes*, not the size of the match set: the
	// MERGE only returns users it inserted or deleted.
	metrics.ObserveDatasetQuery(metrics.QueryListRecompute, projectID, time.Since(start), len(results), err)
	if err != nil {
		return results, err
	}

	// Record the recomputation timestamp so the scheduler can determine when
	// time-dependent lists are next due for reconciliation.
	_, err = s.db.ExecContext(ctx, `UPDATE lists SET last_recomputed_at = NOW() WHERE id = $1`, listID)
	if err != nil {
		return results, err
	}

	return results, nil
}

// ScanListUsers iterates over user IDs in a list and calls fn for each user ID.
// Rows are read via a cursor so large result sets do not need to be held
// entirely in memory.
func (s *ListsStore) ScanListUsers(ctx context.Context, listID uuid.UUID, fn func(uuid.UUID) error) (int, error) {
	q := `
	SELECT user_id
	FROM list_users
	WHERE list_id = $1
	ORDER BY user_id`

	rows, err := s.db.QueryxContext(ctx, q, listID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var n int
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return n, err
		}
		if err := fn(userID); err != nil {
			return n, err
		}
		n++
	}

	return n, rows.Err()
}

// ListUsersBatch returns up to limit user IDs from the list starting at the
// given offset, ordered by user_id. It is used for paginated broadcast fan-out
// so that large lists are processed in manageable chunks.
func (s *ListsStore) ListUsersBatch(ctx context.Context, listID uuid.UUID, limit, offset int) ([]uuid.UUID, error) {
	q := `
	SELECT user_id
	FROM list_users
	WHERE list_id = $1
	ORDER BY user_id
	LIMIT $2 OFFSET $3`

	var userIDs []uuid.UUID
	if err := s.db.SelectContext(ctx, &userIDs, q, listID, limit, offset); err != nil {
		return nil, err
	}

	return userIDs, nil
}

// GetPublishedRule returns the published rule for a list by looking up the
// published version's rule. Returns nil if no published version/rule exists.
func (s *ListsStore) GetPublishedRule(ctx context.Context, listID uuid.UUID) (*rules.RuleSet, error) {
	var ruleset store.JSONB[rules.RuleSet]
	err := s.db.GetContext(ctx, &ruleset, `
		SELECT r.rule
		FROM list_versions lv
		JOIN rules r ON lv.rule_id = r.id
		WHERE lv.list_id = $1 AND lv.status = 'published'
		ORDER BY lv.version_number DESC
		LIMIT 1`, listID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &ruleset.Data, nil
}

// ListDueForReconciliation identifies a list that needs time-based
// recomputation (its recompute interval has elapsed since the last recompute).
type ListDueForReconciliation struct {
	ID        uuid.UUID `db:"id"`
	ProjectID uuid.UUID `db:"project_id"`
}

// SelectListsDueForTimeReconciliation returns dynamic lists whose rules depend
// on rolling time periods and whose recompute interval has elapsed since the
// last recomputation. Lists that have never been recomputed (last_recomputed_at
// IS NULL) are always included.
//
// The query uses FOR UPDATE OF l SKIP LOCKED so that rows already locked by a
// concurrent transaction are silently skipped instead of blocking. This prevents
// double-processing during cluster leader transitions where two nodes may
// briefly both act as leader.
func (s *ListsStore) SelectListsDueForTimeReconciliation(ctx context.Context, limit int) ([]ListDueForReconciliation, error) {
	query := `
	SELECT l.id, l.project_id
	FROM lists l
	JOIN list_versions lv ON lv.id = l.version_id AND lv.status = 'published'
	JOIN rules r ON r.id = lv.rule_id
	WHERE l.type = 'dynamic'
		AND l.deleted_at IS NULL
		AND r.depends_on_time = TRUE
		AND r.recompute_interval IS NOT NULL
		AND (
			l.last_recomputed_at IS NULL
			OR l.last_recomputed_at + r.recompute_interval <= NOW()
		)
	ORDER BY l.last_recomputed_at ASC NULLS FIRST, l.created_at ASC
	LIMIT $1
	FOR UPDATE OF l SKIP LOCKED`

	var results []ListDueForReconciliation
	err := s.db.SelectContext(ctx, &results, query, limit)
	if err != nil {
		return nil, err
	}

	return results, nil
}
