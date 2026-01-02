package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
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

type JourneyVersionStep struct {
	ID         uuid.UUID   `db:"id"`
	VersionID  uuid.UUID   `db:"version_id"`
	ExternalID string      `db:"external_id"`
	Type       string      `db:"type"`
	Name       *string     `db:"name"`
	Data       *JSONB[any] `db:"data"`
	DataKey    *string     `db:"data_key"`
	X          float64     `db:"x"`
	Y          float64     `db:"y"`
	CreatedAt  time.Time   `db:"created_at"`
}

type JourneyVersionSteps []JourneyVersionStep

type JourneyVersionStepChild struct {
	VersionID        uuid.UUID   `db:"version_id"`
	ParentExternalID string      `db:"parent_external_id"`
	ChildExternalID  string      `db:"child_external_id"`
	Path             *string     `db:"path"`
	Data             *JSONB[any] `db:"data"`
}

type JourneyVersionStepChildren []JourneyVersionStepChild

func (steps JourneyVersionSteps) OAPI(children JourneyVersionStepChildren) oapi.JourneyStepMap {
	result := make(oapi.JourneyStepMap)

	for _, step := range steps {
		stepChildren := make([]oapi.JourneyStepChild, 0)
		for _, child := range children {
			if child.ParentExternalID == step.ExternalID {
				childData := oapi.JourneyStepChild{
					ExternalId: child.ChildExternalID,
					Path:       child.Path,
					Data:       child.Data.MarshalRaw(),
				}
				stepChildren = append(stepChildren, childData)
			}
		}

		result[step.ExternalID] = oapi.JourneyStep{
			Type:     oapi.JourneyStepType(step.Type),
			X:        float32(step.X),
			Y:        float32(step.Y),
			Name:     step.Name,
			Data:     step.Data.MarshalRaw(),
			DataKey:  step.DataKey,
			Children: stepChildren,
		}
	}

	return result
}

type JourneyUserState struct {
	ID              uuid.UUID   `db:"id"`
	JourneyID       uuid.UUID   `db:"journey_id"`
	UserID          uuid.UUID   `db:"user_id"`
	PinnedVersionID *uuid.UUID  `db:"pinned_version_id"`
	ExternalID      *string     `db:"external_id"`
	Type            *string     `db:"type"`
	EnteredAt       time.Time   `db:"entered_at"`
	ResumeAt        *time.Time  `db:"resume_at"`
	CompletedAt     *time.Time  `db:"completed_at"`
	Data            *JSONB[any] `db:"data"`
	Status          string      `db:"status"`
	UpdatedAt       time.Time   `db:"updated_at"`
}

func NewJourneysStore(db DB) *JourneysStore {
	return &JourneysStore{db: db}
}

type JourneysStore struct {
	db DB
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

func (s *JourneysStore) ListJourneys(ctx context.Context, projectID uuid.UUID, pagination Pagination) (Journeys, int, error) {
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

func (s *JourneysStore) GetJourneySteps(ctx context.Context, versionID uuid.UUID) (JourneyVersionSteps, error) {
	query := `
	SELECT id, version_id, external_id, type, name, data, data_key, x, y, created_at
	FROM journey_version_steps
	WHERE version_id = $1
	ORDER BY created_at ASC`

	var steps JourneyVersionSteps
	err := s.db.SelectContext(ctx, &steps, query, versionID)
	if err != nil {
		return nil, err
	}

	return steps, nil
}

func (s *JourneysStore) GetJourneyStepChildren(ctx context.Context, versionID uuid.UUID) (JourneyVersionStepChildren, error) {
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

func (s *JourneysStore) SetJourneySteps(ctx context.Context, versionID uuid.UUID, steps oapi.JourneyStepMap) (map[string]uuid.UUID, error) {
	stmt := `
	INSERT INTO journey_version_steps (version_id, external_id, type, name, data, data_key, x, y)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	ON CONFLICT (version_id, external_id) 
	DO UPDATE SET
		type = EXCLUDED.type,
		name = EXCLUDED.name,
		data = EXCLUDED.data,
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

func (s *JourneysStore) GetNextStep(ctx context.Context, userStateID uuid.UUID) (*struct {
	ChildExternalID string
	Path            *string
	Data            *JSONB[any]
	StepID          uuid.UUID
	Type            string
	StepData        *JSONB[any]
}, error) {
	query := `
	WITH user_state AS (
		SELECT
			jus.id,
			jus.journey_id,
			jus.user_id,
			jus.external_id,
			COALESCE(jus.pinned_version_id, j.version_id) AS effective_version_id
		FROM journey_user_state jus
		JOIN journeys j ON j.id = jus.journey_id
		WHERE jus.id = $1
	),
	next_step AS (
		SELECT
			c.child_external_id,
			c.path,
			c.data,
			jvs.id AS step_id,
			jvs.type,
			jvs.data AS step_data
		FROM user_state u
		JOIN journey_version_step_children c
			ON c.version_id = u.effective_version_id
			AND c.parent_external_id = u.external_id
		JOIN journey_version_steps jvs
			ON jvs.version_id = u.effective_version_id
			AND jvs.external_id = c.child_external_id
		LIMIT 1
	)
	SELECT child_external_id, path, data, step_id, type, step_data
	FROM next_step`

	type result struct {
		ChildExternalID string      `db:"child_external_id"`
		Path            *string     `db:"path"`
		Data            *JSONB[any] `db:"data"`
		StepID          uuid.UUID   `db:"step_id"`
		Type            string      `db:"type"`
		StepData        *JSONB[any] `db:"step_data"`
	}

	var res result
	err := s.db.GetContext(ctx, &res, query, userStateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &struct {
		ChildExternalID string
		Path            *string
		Data            *JSONB[any]
		StepID          uuid.UUID
		Type            string
		StepData        *JSONB[any]
	}{
		ChildExternalID: res.ChildExternalID,
		Path:            res.Path,
		Data:            res.Data,
		StepID:          res.StepID,
		Type:            res.Type,
		StepData:        res.StepData,
	}, nil
}

func (s *JourneysStore) CreateUserJourneyState(ctx context.Context, state JourneyUserState) (uuid.UUID, error) {
	stmt := `
	INSERT INTO journey_user_state (journey_id, user_id, pinned_version_id, external_id, type, entered_at, data, status)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt,
		state.JourneyID,
		state.UserID,
		state.PinnedVersionID,
		state.ExternalID,
		state.Type,
		state.EnteredAt,
		state.Data,
		state.Status,
	)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *JourneysStore) GetUserJourneyState(ctx context.Context, journeyID, userID uuid.UUID) (*JourneyUserState, error) {
	stmt := `
	SELECT id, journey_id, user_id, pinned_version_id, external_id, type, entered_at, resume_at, completed_at, data, status, updated_at
	FROM journey_user_state
	WHERE journey_id = $1 AND user_id = $2`

	var state JourneyUserState
	err := s.db.GetContext(ctx, &state, stmt, journeyID, userID)
	if err != nil {
		return nil, err
	}

	return &state, nil
}

func (s *JourneysStore) UpdateUserJourneyState(ctx context.Context, stateID uuid.UUID, update map[string]interface{}) error {
	query := "UPDATE journey_user_state SET"
	args := []interface{}{}
	argCount := 1
	first := true

	for k, v := range update {
		if !first {
			query += ","
		}
		query += fmt.Sprintf(" %s = $%d", k, argCount)
		args = append(args, v)
		argCount++
		first = false
	}

	query += fmt.Sprintf(" WHERE id = $%d", argCount)
	args = append(args, stateID)

	_, err := s.db.ExecContext(ctx, query, args...)
	return err
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

func (s *JourneysStore) ListUserJourneyEntrances(ctx context.Context, projectID, userID uuid.UUID, pagination Pagination) (UserJourneyEntrances, int, error) {
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
