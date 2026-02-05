package journey

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store"
)

type Journeys []Journey

func (j Journeys) OAPI() []oapi.Journey {
	results := make([]oapi.Journey, len(j))
	for i, journey := range j {
		results[i] = journey.OAPI(nil)
	}
	return results
}

func (j Journeys) OAPIWithVersionInfo(versionInfos map[uuid.UUID]*JourneyVersionInfo) []oapi.Journey {
	results := make([]oapi.Journey, len(j))
	for i, journey := range j {
		results[i] = journey.OAPI(versionInfos[journey.ID])
	}
	return results
}

type Journey struct {
	ID          uuid.UUID  `db:"id"`
	ProjectID   uuid.UUID  `db:"project_id"`
	Name        string     `db:"name"`
	Description *string    `db:"description"`
	VersionID   *uuid.UUID `db:"version_id"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
}

func (j *Journey) OAPI(versionInfo *JourneyVersionInfo) oapi.Journey {
	// Default status to draft if no version info
	status := oapi.JourneyStatus("draft")
	var versionNumber *int
	var draftVersionID *uuid.UUID
	var publishedVersionID *uuid.UUID

	if versionInfo != nil {
		status = oapi.JourneyStatus(versionInfo.Status)
		versionNumber = &versionInfo.VersionNumber
		draftVersionID = versionInfo.DraftVersionID
		publishedVersionID = versionInfo.PublishedVersionID
	}

	return oapi.Journey{
		Id:                 j.ID,
		ProjectId:          j.ProjectID,
		Name:               j.Name,
		Description:        j.Description,
		Status:             status,
		VersionNumber:      versionNumber,
		DraftVersionId:     draftVersionID,
		PublishedVersionId: publishedVersionID,
		CreatedAt:          j.CreatedAt,
		UpdatedAt:          j.UpdatedAt,
	}
}

type JourneyVersion struct {
	ID            uuid.UUID  `db:"id"`
	JourneyID     uuid.UUID  `db:"journey_id"`
	VersionNumber int        `db:"version_number"`
	Status        string     `db:"status"`
	CreatedAt     time.Time  `db:"created_at"`
	PublishedAt   *time.Time `db:"published_at"`
}

type JourneyVersionSteps []JourneyVersionStep

func (steps JourneyVersionSteps) OAPI() oapi.JourneyStepMap {
	result := make(oapi.JourneyStepMap)
	for _, step := range steps {
		result[step.ExternalID] = step.OAPI()
	}

	return result
}

type JourneyVersionStep struct {
	ID         uuid.UUID                  `db:"id"`
	VersionID  uuid.UUID                  `db:"version_id"`
	ExternalID string                     `db:"external_id"`
	Type       string                     `db:"type"`
	Name       *string                    `db:"name"`
	Data       json.RawMessage            `db:"data"`
	DataKey    *string                    `db:"data_key"`
	X          float64                    `db:"x"`
	Y          float64                    `db:"y"`
	CreatedAt  time.Time                  `db:"created_at"`
	Children   JourneyVersionStepChildren `db:"children"`
}

func (step JourneyVersionStep) OAPI() oapi.JourneyStep {
	return oapi.JourneyStep{
		Type:     oapi.JourneyStepType(step.Type),
		X:        float32(step.X),
		Y:        float32(step.Y),
		Name:     step.Name,
		Data:     step.Data,
		DataKey:  step.DataKey,
		Children: JourneyVersionStepChildrenOAPI(step.Children),
	}
}

// JourneyVersionStepChildren is an alias for the shared type with OAPI conversion.
type JourneyVersionStepChildren = store.JourneyVersionStepChildren

// JourneyVersionStepChild is an alias for the shared type.
type JourneyVersionStepChild = store.JourneyVersionStepChild

// JourneyVersionStepChildrenOAPI converts children to OAPI format.
func JourneyVersionStepChildrenOAPI(steps JourneyVersionStepChildren) []oapi.JourneyStepChild {
	results := make([]oapi.JourneyStepChild, len(steps))
	for i, child := range steps {
		results[i] = oapi.JourneyStepChild{
			ExternalId: child.ChildExternalID,
			Path:       child.Path,
			Data:       child.Data,
		}
	}
	return results
}

type JourneyUserState struct {
	ID              uuid.UUID       `db:"id"`
	ProjectID       uuid.UUID       `db:"project_id"`
	JourneyID       uuid.UUID       `db:"journey_id"`
	JourneyEntryID  uuid.UUID       `db:"journey_entry_id"`
	UserID          uuid.UUID       `db:"user_id"`
	ExternalStepID  string          `db:"external_step_id"`
	PinnedVersionID *uuid.UUID      `db:"pinned_version_id"`
	Occurrence      int             `db:"occurrence"`
	EnteredAt       time.Time       `db:"entered_at"`
	ResumeAt        *time.Time      `db:"resume_at"`
	CompletedAt     *time.Time      `db:"completed_at"`
	Data            json.RawMessage `db:"data"`
	UpdatedAt       time.Time       `db:"updated_at"`
}

func NewJourneysStore(db store.DB) *JourneysStore {
	return &JourneysStore{db: db}
}

type JourneysStore struct {
	db store.DB
}

func (s *JourneysStore) GetJourney(ctx context.Context, projectID, journeyID uuid.UUID) (*Journey, error) {
	stmt := `
	SELECT id, project_id, name, description, version_id, created_at, updated_at
	FROM journeys
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL`

	var journey Journey
	err := s.db.GetContext(ctx, &journey, stmt, journeyID, projectID)
	if err != nil {
		return nil, err
	}

	return &journey, nil
}

type JourneyVersionInfo struct {
	Status             string     `db:"status"`
	VersionNumber      int        `db:"version_number"`
	DraftVersionID     *uuid.UUID `db:"draft_version_id"`
	PublishedVersionID *uuid.UUID `db:"published_version_id"`
}

func (s *JourneysStore) GetJourneyVersionInfo(ctx context.Context, journeyID uuid.UUID) (*JourneyVersionInfo, error) {
	stmt := `
	SELECT
		COALESCE(active.status, 'draft') as status,
		COALESCE(active.version_number, 0) as version_number,
		draft.id as draft_version_id,
		published.id as published_version_id
	FROM journeys j
	LEFT JOIN journey_versions active ON j.version_id = active.id
	LEFT JOIN journey_versions draft ON j.id = draft.journey_id AND draft.status = 'draft'
	LEFT JOIN (
		SELECT journey_id, MAX(version_number) as max_version
		FROM journey_versions
		WHERE status = 'published'
		GROUP BY journey_id
	) latest_pub ON j.id = latest_pub.journey_id
	LEFT JOIN journey_versions published ON j.id = published.journey_id
		AND published.status = 'published'
		AND published.version_number = latest_pub.max_version
	WHERE j.id = $1 AND j.deleted_at IS NULL`

	var info JourneyVersionInfo
	err := s.db.GetContext(ctx, &info, stmt, journeyID)
	if err != nil {
		return nil, err
	}

	return &info, nil
}

// ResolveVersionID returns the version ID to use for a journey.
// If the journey has a version_id set, it returns that.
// Otherwise, it returns the latest draft version ID.
// Returns sql.ErrNoRows if no version exists.
func (s *JourneysStore) ResolveVersionID(ctx context.Context, journeyID uuid.UUID) (uuid.UUID, error) {
	query := `
	SELECT COALESCE(
		(SELECT version_id FROM journeys WHERE id = $1 AND deleted_at IS NULL),
		(SELECT id FROM journey_versions WHERE journey_id = $1 AND status = 'draft' ORDER BY version_number DESC LIMIT 1)
	) as version_id`

	var versionID *uuid.UUID
	err := s.db.GetContext(ctx, &versionID, query, journeyID)
	if err != nil {
		return uuid.Nil, err
	}
	if versionID == nil {
		return uuid.Nil, sql.ErrNoRows
	}
	return *versionID, nil
}

// EnsureDraftVersion returns a draft version ID for the journey.
// If a draft version exists for the journey, returns that.
// Otherwise, creates a new draft version and returns its ID.
// When creating a new draft, it copies steps from the current published version.
func (s *JourneysStore) EnsureDraftVersion(ctx context.Context, journeyID uuid.UUID) (uuid.UUID, error) {
	// Check if ANY draft version exists for this journey
	query := `
	SELECT id
	FROM journey_versions
	WHERE journey_id = $1 AND status = 'draft'
	LIMIT 1`

	var draftID uuid.UUID
	err := s.db.GetContext(ctx, &draftID, query, journeyID)
	if err == nil {
		// Draft exists, update journey to point to it
		updateStmt := `UPDATE journeys SET version_id = $1 WHERE id = $2`
		_, err = s.db.ExecContext(ctx, updateStmt, draftID, journeyID)
		if err != nil {
			return uuid.Nil, err
		}
		return draftID, nil
	}
	if err != sql.ErrNoRows {
		return uuid.Nil, err
	}

	// No draft exists, create new one
	newDraftID, err := s.CreateJourneyVersion(ctx, journeyID, "draft")
	if err != nil {
		return uuid.Nil, err
	}

	// Copy steps from current published version if it exists
	currentVersionID, err := s.ResolveVersionID(ctx, journeyID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, err
	}

	// If there's a current version, copy its steps
	if err == nil && currentVersionID != uuid.Nil {
		err = s.CopyVersionSteps(ctx, currentVersionID, newDraftID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("failed to copy steps to new draft: %w", err)
		}
	}

	// Update journey to point to the new draft
	updateStmt := `UPDATE journeys SET version_id = $1 WHERE id = $2`
	_, err = s.db.ExecContext(ctx, updateStmt, newDraftID, journeyID)
	if err != nil {
		return uuid.Nil, err
	}

	return newDraftID, nil
}

func (s *JourneysStore) CreateJourney(ctx context.Context, journey Journey) (uuid.UUID, error) {
	stmt := `
	INSERT INTO journeys (project_id, name, description)
	VALUES ($1, $2, $3)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt,
		journey.ProjectID,
		journey.Name,
		journey.Description,
	)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *JourneysStore) ListJourneys(ctx context.Context, projectID uuid.UUID, pagination store.Pagination) (Journeys, int, error) {
	query := `
	SELECT
		id,
		project_id,
		name,
		description,
		version_id,
		created_at,
		updated_at,
		COUNT(*) OVER () AS total_count
	FROM journeys
	WHERE project_id = $1 AND deleted_at IS NULL
	ORDER BY created_at DESC
	LIMIT $2 OFFSET $3`

	type rows struct {
		Journey
		TotalCount int `db:"total_count"`
	}

	var results []rows
	err := s.db.SelectContext(ctx, &results, query, projectID, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []Journey{}, 0, nil
	}

	total := results[0].TotalCount

	journeys := make([]Journey, len(results))
	for i, r := range results {
		journeys[i] = r.Journey
	}

	return journeys, total, nil
}

func (s *JourneysStore) GetJourneyVersionInfoMap(ctx context.Context, journeyIDs []uuid.UUID) (map[uuid.UUID]*JourneyVersionInfo, error) {
	if len(journeyIDs) == 0 {
		return make(map[uuid.UUID]*JourneyVersionInfo), nil
	}

	query := `
	SELECT
		j.id as journey_id,
		COALESCE(active.status, 'draft') as status,
		COALESCE(active.version_number, 0) as version_number,
		draft.id as draft_version_id,
		published.id as published_version_id
	FROM UNNEST($1::uuid[]) AS j(id)
	LEFT JOIN journeys ON j.id = journeys.id
	LEFT JOIN journey_versions active ON journeys.version_id = active.id
	LEFT JOIN journey_versions draft ON journeys.id = draft.journey_id AND draft.status = 'draft'
	LEFT JOIN (
		SELECT journey_id, MAX(version_number) as max_version
		FROM journey_versions
		WHERE status = 'published'
		GROUP BY journey_id
	) latest_pub ON journeys.id = latest_pub.journey_id
	LEFT JOIN journey_versions published ON journeys.id = published.journey_id
		AND published.status = 'published'
		AND published.version_number = latest_pub.max_version`

	type row struct {
		JourneyID uuid.UUID `db:"journey_id"`
		JourneyVersionInfo
	}

	var rows []row
	err := s.db.SelectContext(ctx, &rows, query, journeyIDs)
	if err != nil {
		return nil, err
	}

	result := make(map[uuid.UUID]*JourneyVersionInfo, len(rows))
	for _, r := range rows {
		info := r.JourneyVersionInfo
		result[r.JourneyID] = &info
	}

	return result, nil
}

type JourneyUpdate struct {
	Name        *string
	Description *string
	VersionID   *uuid.UUID
}

func (s *JourneysStore) UpdateJourney(ctx context.Context, projectID, journeyID uuid.UUID, update JourneyUpdate) error {
	stmt := `
	UPDATE journeys
	SET
		name = COALESCE($1, name),
		description = COALESCE($2, description),
		version_id = COALESCE($3, version_id)
	WHERE id = $4 AND project_id = $5 AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, update.Name, update.Description, update.VersionID, journeyID, projectID)
	return err
}

func (s *JourneysStore) DeleteJourney(ctx context.Context, projectID, journeyID uuid.UUID) error {
	stmt := `
	UPDATE journeys
	SET deleted_at = now()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, journeyID, projectID)
	return err
}

func (s *JourneysStore) CreateJourneyVersion(ctx context.Context, journeyID uuid.UUID, status string) (uuid.UUID, error) {
	stmt := `
	WITH next_version AS (
		SELECT COALESCE(MAX(version_number), 0) + 1 as next_num
		FROM journey_versions
		WHERE journey_id = $1
	)
	INSERT INTO journey_versions (journey_id, version_number, status)
	SELECT $1, next_num, $2
	FROM next_version
	RETURNING id`

	var versionID uuid.UUID
	err := s.db.GetContext(ctx, &versionID, stmt, journeyID, status)
	if err != nil {
		return uuid.Nil, err
	}

	return versionID, nil
}

func (s *JourneysStore) GetJourneyVersion(ctx context.Context, versionID uuid.UUID) (*JourneyVersion, error) {
	stmt := `
	SELECT id, journey_id, version_number, status, created_at, published_at
	FROM journey_versions
	WHERE id = $1`

	var version JourneyVersion
	err := s.db.GetContext(ctx, &version, stmt, versionID)
	if err != nil {
		return nil, err
	}

	return &version, nil
}

func (s *JourneysStore) GetCurrentVersion(ctx context.Context, journeyID uuid.UUID) (*JourneyVersion, error) {
	stmt := `
	SELECT jv.id, jv.journey_id, jv.version_number, jv.status, jv.created_at, jv.published_at
	FROM journey_versions jv
	JOIN journeys j ON j.version_id = jv.id
	WHERE j.id = $1`

	var version JourneyVersion
	err := s.db.GetContext(ctx, &version, stmt, journeyID)
	if err != nil {
		return nil, err
	}

	return &version, nil
}

func (s *JourneysStore) GetLatestDraftVersion(ctx context.Context, journeyID uuid.UUID) (*JourneyVersion, error) {
	stmt := `
	SELECT id, journey_id, version_number, status, created_at, published_at
	FROM journey_versions
	WHERE journey_id = $1 AND status = 'draft'
	ORDER BY version_number DESC
	LIMIT 1`

	var version JourneyVersion
	err := s.db.GetContext(ctx, &version, stmt, journeyID)
	if err != nil {
		return nil, err
	}

	return &version, nil
}

func (s *JourneysStore) CopyVersionSteps(ctx context.Context, fromVersionID, toVersionID uuid.UUID) error {
	// Copy steps
	copyStepsStmt := `
	INSERT INTO journey_version_steps (version_id, external_id, type, name, data, data_key, x, y)
	SELECT $1, external_id, type, name, data, data_key, x, y
	FROM journey_version_steps
	WHERE version_id = $2`

	_, err := s.db.ExecContext(ctx, copyStepsStmt, toVersionID, fromVersionID)
	if err != nil {
		return fmt.Errorf("failed to copy steps: %w", err)
	}

	// Copy step children (connections)
	copyChildrenStmt := `
	INSERT INTO journey_version_step_children (version_id, parent_external_id, child_external_id, path, data)
	SELECT $1, parent_external_id, child_external_id, path, data
	FROM journey_version_step_children
	WHERE version_id = $2`

	_, err = s.db.ExecContext(ctx, copyChildrenStmt, toVersionID, fromVersionID)
	if err != nil {
		return fmt.Errorf("failed to copy step children: %w", err)
	}

	// Copy event dependencies
	copyDepsStmt := `
	INSERT INTO journey_version_step_events (version_id, external_id, event_id)
	SELECT $1, external_id, event_id
	FROM journey_version_step_events
	WHERE version_id = $2`

	_, err = s.db.ExecContext(ctx, copyDepsStmt, toVersionID, fromVersionID)
	if err != nil {
		return fmt.Errorf("failed to copy event dependencies: %w", err)
	}

	return nil
}

func (s *JourneysStore) PublishVersion(ctx context.Context, journeyID, versionID uuid.UUID) error {
	archiveStmt := `
	UPDATE journey_versions
	SET status = 'archived'
	WHERE journey_id = $1 AND status = 'published'`

	_, err := s.db.ExecContext(ctx, archiveStmt, journeyID)
	if err != nil {
		return err
	}

	publishStmt := `
	UPDATE journey_versions
	SET status = 'published', published_at = now()
	WHERE id = $1 AND journey_id = $2`

	_, err = s.db.ExecContext(ctx, publishStmt, versionID, journeyID)
	if err != nil {
		return err
	}

	updateJourneyStmt := `
	UPDATE journeys
	SET version_id = $1
	WHERE id = $2`

	_, err = s.db.ExecContext(ctx, updateJourneyStmt, versionID, journeyID)
	return err
}

func (s *JourneysStore) GetJourneyVersionStepsChildren(ctx context.Context, versionID uuid.UUID) (JourneyVersionStepChildren, error) {
	query := `
	SELECT version_id, parent_external_id, child_external_id, path, data
	FROM journey_version_step_children
	WHERE version_id = $1`

	var children JourneyVersionStepChildren
	err := s.db.SelectContext(ctx, &children, query, versionID)
	if err != nil {
		return nil, err
	}

	return children, nil
}

func (s *JourneysStore) GetJourneyStep(ctx context.Context, journeyID uuid.UUID, externalID string, versionID *uuid.UUID) (JourneyVersionStep, error) {
	query := `
	WITH target_version AS (
		SELECT COALESCE($3, j.version_id) AS version_id
		FROM journeys j
		WHERE j.id = $1
	)
	SELECT
		s.id,
		s.version_id,
		s.external_id,
		s.type,
		s.name,
		s.data,
		s.data_key,
		s.x,
		s.y,
		s.created_at,
		COALESCE(
			json_agg(row_to_json(c)) FILTER (WHERE c.version_id IS NOT NULL),
			'[]'
		) AS children
	FROM target_version v
	JOIN journey_version_steps s ON s.version_id = v.version_id
	LEFT JOIN journey_version_step_children c ON c.version_id = v.version_id AND s.external_id = c.parent_external_id
	WHERE s.external_id = $2
	GROUP BY s.id
	ORDER BY s.created_at ASC`

	var step JourneyVersionStep
	err := s.db.GetContext(ctx, &step, query, journeyID, externalID, versionID)
	if err != nil {
		return JourneyVersionStep{}, err
	}

	return step, nil
}

func (s *JourneysStore) GetJourneyVersionSteps(ctx context.Context, versionID uuid.UUID) (JourneyVersionSteps, error) {
	query := `
	SELECT
		s.id,
		s.version_id,
		s.external_id,
		s.type,
		s.name,
		s.data,
		s.data_key,
		s.x,
		s.y,
		s.created_at,
		COALESCE(
			json_agg(row_to_json(c)) FILTER (WHERE c.version_id IS NOT NULL),
			'[]'
		) AS children
	FROM journey_version_steps s
	LEFT JOIN journey_version_step_children c ON c.version_id = s.version_id AND s.external_id = c.parent_external_id
	WHERE s.version_id = $1
	GROUP BY s.id
	ORDER BY s.created_at ASC`

	var steps JourneyVersionSteps
	err := s.db.SelectContext(ctx, &steps, query, versionID)
	if err != nil {
		return nil, err
	}

	return steps, nil
}

func (s *JourneysStore) SetJourneySteps(ctx context.Context, versionID uuid.UUID, steps oapi.JourneyStepMap) (map[string]uuid.UUID, error) {
	stmt := `
	INSERT INTO journey_version_steps (version_id, external_id, type, name, data, data_key, x, y)
	VALUES ($1, $2, $3, $4, COALESCE($5, '{}'::jsonb), $6, $7, $8)
	ON CONFLICT (version_id, external_id)
	DO UPDATE SET
		type = EXCLUDED.type,
		name = EXCLUDED.name,
		data = COALESCE(EXCLUDED.data, journey_version_steps.data),
		data_key = EXCLUDED.data_key,
		x = EXCLUDED.x,
		y = EXCLUDED.y
	RETURNING id`

	result := make(map[string]uuid.UUID, len(steps))

	for externalID, step := range steps {
		var stepID uuid.UUID
		err := s.db.GetContext(ctx, &stepID, stmt,
			versionID,
			externalID,
			step.Type,
			step.Name,
			step.Data,
			step.DataKey,
			step.X,
			step.Y,
		)
		if err != nil {
			return nil, err
		}

		result[externalID] = stepID
	}

	externalIDs := make([]string, 0, len(result))
	for k := range result {
		externalIDs = append(externalIDs, k)
	}

	err := s.RemoveOrphanedSteps(ctx, versionID, externalIDs)
	if err != nil {
		return nil, err
	}

	err = s.SetJourneyStepChildren(ctx, versionID, steps)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *JourneysStore) RemoveOrphanedSteps(ctx context.Context, versionID uuid.UUID, keepExternalIDs []string) error {
	deleteStmt := `
	DELETE FROM journey_version_steps
	WHERE version_id = $1 AND external_id != ALL($2)`

	_, err := s.db.ExecContext(ctx, deleteStmt, versionID, keepExternalIDs)
	return err
}

func (s *JourneysStore) SetJourneyStepChildren(ctx context.Context, versionID uuid.UUID, steps oapi.JourneyStepMap) error {
	deleteStmt := `DELETE FROM journey_version_step_children WHERE version_id = $1`
	_, err := s.db.ExecContext(ctx, deleteStmt, versionID)
	if err != nil {
		return err
	}

	if len(steps) == 0 {
		return nil
	}

	stmt := `
	INSERT INTO journey_version_step_children (version_id, parent_external_id, child_external_id, path, data)
	VALUES ($1, $2, $3, $4, $5)`

	for externalID, step := range steps {
		for _, child := range step.Children {
			_, err := s.db.ExecContext(ctx, stmt,
				versionID,
				externalID,
				child.ExternalId,
				child.Path,
				child.Data,
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (s *JourneysStore) SetJourneyStepEventDependencies(ctx context.Context, versionID uuid.UUID, externalID string, eventIDs []uuid.UUID) error {
	deleteStmt := `
	DELETE FROM journey_version_step_events
	WHERE version_id = $1 AND external_id = $2`

	_, err := s.db.ExecContext(ctx, deleteStmt, versionID, externalID)
	if err != nil {
		return err
	}

	if len(eventIDs) == 0 {
		return nil
	}

	stmt := `
	INSERT INTO journey_version_step_events (version_id, external_id, event_id)
	VALUES ($1, $2, $3)`

	for _, eventID := range eventIDs {
		_, err = s.db.ExecContext(ctx, stmt, versionID, externalID, eventID)
		if err != nil {
			return err
		}
	}

	return nil
}

// DuplicateJourney creates a new draft version by copying an existing version.
// If asVersion is true, creates a new version for the same journey.
// If asVersion is false, creates a completely new journey with copied content.
func (s *JourneysStore) DuplicateJourney(ctx context.Context, journey *Journey, asVersion bool) (uuid.UUID, error) {
	if journey.VersionID == nil {
		return uuid.Nil, errors.New("journey has no version to duplicate")
	}

	var journeyID uuid.UUID
	var err error

	if asVersion {
		journeyID = journey.ID
	} else {
		journeyID, err = s.CreateJourney(ctx, Journey{
			ProjectID:   journey.ProjectID,
			Name:        "Copy of " + journey.Name,
			Description: journey.Description,
		})
		if err != nil {
			return uuid.Nil, err
		}
	}

	newVersionID, err := s.CreateJourneyVersion(ctx, journeyID, "draft")
	if err != nil {
		return uuid.Nil, err
	}

	if !asVersion {
		_, err = s.db.ExecContext(ctx, `UPDATE journeys SET version_id = $1 WHERE id = $2`, newVersionID, journeyID)
		if err != nil {
			return uuid.Nil, err
		}
	}

	err = s.CopyVersionContent(ctx, *journey.VersionID, newVersionID)
	if err != nil {
		return uuid.Nil, err
	}

	return journeyID, nil
}

func (s *JourneysStore) CopyVersionContent(ctx context.Context, sourceVersionID, targetVersionID uuid.UUID) error {
	copySteps := `
	INSERT INTO journey_version_steps (version_id, external_id, type, name, data, data_key, x, y)
	SELECT $1, external_id, type, name, data, data_key, x, y
	FROM journey_version_steps
	WHERE version_id = $2`

	_, err := s.db.ExecContext(ctx, copySteps, targetVersionID, sourceVersionID)
	if err != nil {
		return err
	}

	copyChildren := `
	INSERT INTO journey_version_step_children (version_id, parent_external_id, child_external_id, path, data)
	SELECT $1, parent_external_id, child_external_id, path, data
	FROM journey_version_step_children
	WHERE version_id = $2`

	_, err = s.db.ExecContext(ctx, copyChildren, targetVersionID, sourceVersionID)
	if err != nil {
		return err
	}

	copyEvents := `
	INSERT INTO journey_version_step_events (version_id, external_id, event_id)
	SELECT $1, external_id, event_id
	FROM journey_version_step_events
	WHERE version_id = $2`

	_, err = s.db.ExecContext(ctx, copyEvents, targetVersionID, sourceVersionID)
	return err
}

func (s *JourneysStore) CreateUserJourneyState(ctx context.Context, state JourneyUserState) (uuid.UUID, error) {
	stmt := `
	INSERT INTO journey_user_state (id, journey_id, journey_entry_id, user_id, external_step_id, pinned_version_id, occurrence, data, completed_at, resume_at)
	VALUES (
		COALESCE($1, uuid_generate_v4()),
		$2, $3, $4, $5, $6,
		COALESCE(
			(SELECT MAX(occurrence) + 1 FROM journey_user_state WHERE journey_entry_id = $3 AND external_step_id = $5),
			1
		),
		COALESCE($7, '{}'::jsonb), $8, $9
	)
	ON CONFLICT (id) DO UPDATE SET
		data = EXCLUDED.data,
		completed_at = EXCLUDED.completed_at,
		resume_at = EXCLUDED.resume_at
	RETURNING id`

	var id *uuid.UUID
	if state.ID != uuid.Nil {
		id = &state.ID
	}

	err := s.db.GetContext(ctx, &id, stmt,
		id,
		state.JourneyID,
		state.JourneyEntryID,
		state.UserID,
		state.ExternalStepID,
		state.PinnedVersionID,
		state.Data,
		state.CompletedAt,
		state.ResumeAt,
	)
	if err != nil {
		return uuid.Nil, err
	}

	return *id, nil
}

func (s *JourneysStore) ListResumeableUserJourneys(ctx context.Context) ([]JourneyUserState, error) {
	query := `
	SELECT jus.id, j.project_id, jus.journey_id, jus.user_id, jus.pinned_version_id, jus.occurrence, jus.entered_at, jus.resume_at, jus.completed_at, COALESCE(jus.data, '{}'::jsonb) AS data, jus.updated_at, jus.journey_entry_id, jus.external_step_id
	FROM journey_user_state jus
	JOIN journeys j ON j.id = jus.journey_id
	WHERE jus.completed_at IS NULL
	AND jus.resume_at <= NOW()`

	var states []JourneyUserState
	err := s.db.SelectContext(ctx, &states, query)
	if err != nil {
		return nil, err
	}

	return states, nil
}

func (s *JourneysStore) GetJourneyStateByID(ctx context.Context, stateID uuid.UUID) (*JourneyUserState, error) {
	query := `
	SELECT jus.id, j.project_id, jus.journey_id, jus.user_id, jus.pinned_version_id, jus.occurrence, jus.entered_at, jus.resume_at, jus.completed_at, COALESCE(jus.data, '{}'::jsonb) AS data, jus.updated_at, jus.journey_entry_id, jus.external_step_id
	FROM journey_user_state jus
	JOIN journeys j ON j.id = jus.journey_id
	WHERE jus.id = $1`

	var state JourneyUserState
	err := s.db.GetContext(ctx, &state, query, stateID)
	if err != nil {
		return nil, err
	}

	return &state, nil
}

func (s *JourneysStore) GetUserJourneyState(ctx context.Context, journeyEntryID uuid.UUID, externalStepID string) (*JourneyUserState, error) {
	query := `
	SELECT jus.id, j.project_id, jus.journey_id, jus.user_id, jus.pinned_version_id, jus.occurrence, jus.entered_at, jus.resume_at, jus.completed_at, COALESCE(jus.data, '{}'::jsonb) AS data, jus.updated_at, jus.journey_entry_id, jus.external_step_id
	FROM journey_user_state jus
	JOIN journeys j ON j.id = jus.journey_id
	WHERE jus.journey_entry_id = $1
	AND jus.external_step_id = $2
	ORDER BY jus.occurrence DESC
	LIMIT 1`

	var state JourneyUserState
	err := s.db.GetContext(ctx, &state, query, journeyEntryID, externalStepID)
	if err != nil {
		return nil, err
	}

	return &state, nil
}

func (s *JourneysStore) GetJourneyEntryData(ctx context.Context, journeyEntryID, userID uuid.UUID) (map[string]any, error) {
	query := `
	WITH user_json AS (
		SELECT to_jsonb(u) AS user_data
		FROM users u
		WHERE u.id = $2
	),

	journey_state AS (
		SELECT step.data_key, jus.data AS state_data
		FROM journey_user_state jus
		JOIN journeys j ON j.id = jus.journey_id
		JOIN journey_version_steps step ON step.external_id = jus.external_step_id AND step.version_id = COALESCE(jus.pinned_version_id, j.version_id)
		WHERE jus.journey_entry_id = $1
		AND step.data_key IS NOT NULL
	),

	journey_json AS (
		SELECT
			COALESCE(
			jsonb_object_agg(js.data_key, js.state_data),
			'{}'::jsonb
			) AS journey_data
		FROM journey_state js
	)

	SELECT jsonb_build_object(
		'user', uj.user_data,
		'journey', jj.journey_data
	)
	FROM user_json uj
	CROSS JOIN journey_json jj`

	var rawResult json.RawMessage
	err := s.db.GetContext(ctx, &rawResult, query, journeyEntryID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	var result map[string]any
	if err := json.Unmarshal(rawResult, &result); err != nil {
		return nil, err
	}

	return result, nil
}

type UserJourneyEntrances []UserJourneyEntrance

type UserJourneyEntrance struct {
	ID         uuid.UUID  `db:"id"`
	EntranceID uuid.UUID  `db:"entrance_id"`
	Journey    *Journey   `db:"-"`
	CreatedAt  time.Time  `db:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at"`
	EndedAt    *time.Time `db:"ended_at"`
}

func (e UserJourneyEntrances) OAPI() []oapi.UserJourneyEntrance {
	results := make([]oapi.UserJourneyEntrance, len(e))
	for i, entrance := range e {
		results[i] = entrance.OAPI()
	}
	return results
}

func (e *UserJourneyEntrance) OAPI() oapi.UserJourneyEntrance {
	oapiEntrance := oapi.UserJourneyEntrance{
		Id:         e.ID,
		EntranceId: e.EntranceID,
		CreatedAt:  e.CreatedAt,
		UpdatedAt:  e.UpdatedAt,
		EndedAt:    e.EndedAt,
	}

	if e.Journey != nil {
		journey := e.Journey.OAPI(nil)
		oapiEntrance.Journey = &journey
	}

	return oapiEntrance
}

func (s *JourneysStore) ListUserJourneyEntrances(ctx context.Context, projectID, userID uuid.UUID, pagination store.Pagination) (UserJourneyEntrances, int, error) {
	query := `
	SELECT
		jus.id,
		jus.id AS entrance_id,
		jus.entered_at AS created_at,
		jus.updated_at,
		jus.completed_at AS ended_at,
		j.id AS journey_id,
		j.name AS journey_name,
		j.description AS journey_description,
		j.version_id AS journey_version_id,
		j.project_id AS journey_project_id,
		j.created_at AS journey_created_at,
		j.updated_at AS journey_updated_at,
		COUNT(*) OVER () AS total_count
	FROM journey_user_state jus
	LEFT JOIN journeys j ON j.id = jus.journey_id AND j.project_id = $2 AND j.deleted_at IS NULL
	WHERE jus.user_id = $1
		AND jus.journey_id IN (
			SELECT id FROM journeys
			WHERE project_id = $2 AND deleted_at IS NULL
		)
	ORDER BY jus.entered_at DESC
	LIMIT $3 OFFSET $4`

	type row struct {
		UserJourneyEntrance
		JourneyID      *uuid.UUID `db:"journey_id"`
		JourneyName    *string    `db:"journey_name"`
		JourneyDesc    *string    `db:"journey_description"`
		JourneyVersion *uuid.UUID `db:"journey_version_id"`
		JourneyProject *uuid.UUID `db:"journey_project_id"`
		JourneyCreated *time.Time `db:"journey_created_at"`
		JourneyUpdated *time.Time `db:"journey_updated_at"`
		TotalCount     int        `db:"total_count"`
	}

	var rows []row
	err := s.db.SelectContext(ctx, &rows, query, userID, projectID, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	if len(rows) == 0 {
		return []UserJourneyEntrance{}, 0, nil
	}

	total := rows[0].TotalCount

	entrances := make([]UserJourneyEntrance, len(rows))
	for i, r := range rows {
		entrance := r.UserJourneyEntrance

		if r.JourneyID != nil && r.JourneyName != nil {
			entrance.Journey = &Journey{
				ID:          *r.JourneyID,
				ProjectID:   *r.JourneyProject,
				Name:        *r.JourneyName,
				Description: r.JourneyDesc,
				VersionID:   r.JourneyVersion,
				CreatedAt:   *r.JourneyCreated,
				UpdatedAt:   *r.JourneyUpdated,
			}
		}
		entrances[i] = entrance
	}

	return entrances, total, nil
}

func (s *JourneysStore) CompleteJourneyEntryStates(ctx context.Context, journeyID uuid.UUID, entranceExternalID string, completedAt time.Time) error {
	stmt := `
	UPDATE journey_user_state
	SET completed_at = $1, resume_at = NULL
	WHERE journey_entry_id = ANY(SELECT journey_entry_id FROM journey_user_state WHERE external_step_id = $2 AND journey_id = $3)
	AND completed_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, completedAt, entranceExternalID, journeyID)
	return err
}
