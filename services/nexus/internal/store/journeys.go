package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
)

type Journeys []Journey

func (j Journeys) OAPI() []oapi.Journey {
	results := make([]oapi.Journey, len(j))
	for i, journey := range j {
		results[i] = journey.OAPI()
	}
	return results
}

type Journey struct {
	ID          uuid.UUID  `db:"id"`
	ProjectID   uuid.UUID  `db:"project_id"`
	Name        string     `db:"name"`
	Description *string    `db:"description"`
	Status      *string    `db:"status"`
	CreatedAt   time.Time  `db:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at"`
	DeletedAt   *time.Time `db:"deleted_at"`
}

func (j *Journey) OAPI() oapi.Journey {
	status := ""
	if j.Status != nil {
		status = *j.Status
	}

	description := ""
	if j.Description != nil {
		description = *j.Description
	}

	return oapi.Journey{
		Id:          j.ID,
		ProjectId:   j.ProjectID,
		Name:        j.Name,
		Description: &description,
		Status:      status,
		CreatedAt:   j.CreatedAt,
		UpdatedAt:   j.UpdatedAt,
	}
}

type UserJourneyEntrances []UserJourneyEntrance

func (e UserJourneyEntrances) OAPI() []oapi.UserJourneyEntrance {
	results := make([]oapi.UserJourneyEntrance, len(e))
	for i, entrance := range e {
		results[i] = entrance.OAPI()
	}
	return results
}

type UserJourneyEntrance struct {
	ID         uuid.UUID  `db:"id"`
	EntranceID uuid.UUID  `db:"entrance_id"`
	Journey    *Journey   `db:"-"`
	CreatedAt  time.Time  `db:"created_at"`
	UpdatedAt  time.Time  `db:"updated_at"`
	EndedAt    *time.Time `db:"ended_at"`
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
		journey := e.Journey.OAPI()
		oapiEntrance.Journey = &journey
	}

	return oapiEntrance
}

func NewJourneysStore(db DB) *JourneysStore {
	return &JourneysStore{db: db}
}

type JourneysStore struct {
	db DB
}

func (s *JourneysStore) GetJourney(ctx context.Context, projectID, journeyID uuid.UUID) (*Journey, error) {
	stmt := `
	SELECT id, project_id, name, description, status, created_at, updated_at, deleted_at
	FROM journeys
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL`

	var journey Journey
	err := s.db.GetContext(ctx, &journey, stmt, journeyID, projectID)
	if err != nil {
		return nil, err
	}

	return &journey, nil
}

func (s *JourneysStore) CreateJourney(ctx context.Context, journey Journey) (uuid.UUID, error) {
	stmt := `
	INSERT INTO journeys (project_id, name, description, status)
	VALUES ($1, $2, $3, $4)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt,
		journey.ProjectID,
		journey.Name,
		journey.Description,
		journey.Status,
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
		status, 
		created_at, 
		updated_at,
		COUNT(*) OVER () AS total_count
	FROM journeys
	WHERE project_id = $1 AND deleted_at IS NULL
	ORDER BY created_at DESC
	LIMIT $2 OFFSET $3`

	type journeyWithCount struct {
		Journey
		TotalCount int `db:"total_count"`
	}

	var results []journeyWithCount
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

type JourneyUpdate struct {
	Name        *string
	Description *string
	Status      *string
}

func (s *JourneysStore) UpdateJourney(ctx context.Context, projectID, journeyID uuid.UUID, update JourneyUpdate) error {
	stmt := `
	UPDATE journeys
	SET
		name = COALESCE($1, name),
		description = COALESCE($2, description),
		status = COALESCE($3, status)
	WHERE id = $4 AND project_id = $5 AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, update.Name, update.Description, update.Status, journeyID, projectID)
	return err
}

func (s *JourneysStore) DeleteJourney(ctx context.Context, projectID, journeyID uuid.UUID) error {
	stmt := `
	UPDATE journeys
	SET deleted_at = CURRENT_TIMESTAMP
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, journeyID, projectID)
	return err
}

func (s *JourneysStore) CreateUserJourneyStep(ctx context.Context, userID, journeyID uuid.UUID, stepType string) (uuid.UUID, error) {
	stmt := `
	INSERT INTO journey_user_step (user_id, journey_id, entrance_id, type)
	VALUES ($1, $2, NULL, $3)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt, userID, journeyID, stepType)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *JourneysStore) ListUserJourneyEntrances(ctx context.Context, projectID, userID uuid.UUID, pagination Pagination) (UserJourneyEntrances, int, error) {
	query := `
	SELECT 
		jus.id,
		jus.id AS entrance_id,
		jus.journey_id,
		jus.created_at,
		jus.updated_at,
		jus.ended_at,
		COUNT(*) OVER () AS total_count
	FROM journey_user_step jus
	WHERE jus.user_id = $1 
		AND jus.entrance_id IS NULL
		AND EXISTS (
			SELECT 1 FROM journeys j 
			WHERE j.id = jus.journey_id 
			AND j.project_id = $2 
			AND j.deleted_at IS NULL
		)
	ORDER BY jus.created_at DESC
	LIMIT $3 OFFSET $4`

	type result struct {
		ID         uuid.UUID  `db:"id"`
		EntranceID uuid.UUID  `db:"entrance_id"`
		JourneyID  *uuid.UUID `db:"journey_id"`
		CreatedAt  time.Time  `db:"created_at"`
		UpdatedAt  time.Time  `db:"updated_at"`
		EndedAt    *time.Time `db:"ended_at"`
		TotalCount int        `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, userID, projectID, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []UserJourneyEntrance{}, 0, nil
	}

	total := results[0].TotalCount

	// Get unique journey IDs
	journeyIDs := make([]uuid.UUID, 0)
	journeyIDSet := make(map[uuid.UUID]bool)
	for _, r := range results {
		if r.JourneyID != nil && !journeyIDSet[*r.JourneyID] {
			journeyIDs = append(journeyIDs, *r.JourneyID)
			journeyIDSet[*r.JourneyID] = true
		}
	}

	// Fetch journeys
	journeyMap := make(map[uuid.UUID]*Journey)
	if len(journeyIDs) > 0 {
		journeyQuery := `
		SELECT id, project_id, name, description, status, created_at, updated_at, deleted_at
		FROM journeys
		WHERE id = ANY($1) AND project_id = $2 AND deleted_at IS NULL`

		var journeys []Journey
		err = s.db.SelectContext(ctx, &journeys, journeyQuery, journeyIDs, projectID)
		if err != nil {
			return nil, 0, err
		}

		for i := range journeys {
			journeyMap[journeys[i].ID] = &journeys[i]
		}
	}

	// Build result
	entrances := make([]UserJourneyEntrance, len(results))
	for i, r := range results {
		entrance := UserJourneyEntrance{
			ID:         r.ID,
			EntranceID: r.EntranceID,
			CreatedAt:  r.CreatedAt,
			UpdatedAt:  r.UpdatedAt,
			EndedAt:    r.EndedAt,
		}

		if r.JourneyID != nil {
			entrance.Journey = journeyMap[*r.JourneyID]
		}

		entrances[i] = entrance
	}

	return entrances, total, nil
}
