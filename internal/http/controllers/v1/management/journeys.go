package v1

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

func NewJourneysController(logger *zap.Logger, journeyDB, subjectsDB *sqlx.DB, mgmt *management.State) *JourneysController {
	return &JourneysController{
		logger:     logger,
		journeyDB:  journeyDB,
		subjectsDB: subjectsDB,
		mgmt:       mgmt,
		jrny:       journey.NewState(journeyDB),
	}
}

type JourneysController struct {
	logger     *zap.Logger
	journeyDB  *sqlx.DB
	subjectsDB *sqlx.DB
	mgmt       *management.State
	jrny       *journey.State
}

func (srv *JourneysController) ListJourneys(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListJourneysParams) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID))

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	search := params.Search.ToString()

	logger.Info("listing journeys", zap.Int("limit", pagination.Limit), zap.Int("offset", pagination.Offset))

	journeys, total, err := srv.jrny.ListJourneys(ctx, projectID, pagination, search)
	if err != nil {
		logger.Error("failed to list journeys", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Get version info for all journeys
	var journeyIDs []uuid.UUID
	for _, j := range journeys {
		journeyIDs = append(journeyIDs, j.ID)
	}

	versionInfoMap, err := srv.jrny.GetJourneyVersionInfoMap(ctx, journeyIDs)
	if err != nil {
		logger.Error("failed to get version info map", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("journeys listed", zap.Int("total", total), zap.Int("count", len(journeys)))

	response := oapi.JourneyListResponse{
		Results: journeys.OAPIWithVersionInfo(versionInfoMap),
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
	}

	json.Write(w, http.StatusOK, response)
}

func (srv *JourneysController) CreateJourney(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.CreateJourneyParams) {
	ctx := r.Context()
	body := oapi.CreateJourneyJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("name", body.Name))
	logger.Info("creating journey")

	project, err := srv.mgmt.GetProject(ctx, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("project not found", zap.Stringer("project_id", projectID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("project not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get project", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	tx, err := srv.journeyDB.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	defer tx.Rollback() //nolint:errcheck

	journeys := journey.NewJourneysStore(tx)

	journeyID, err := journeys.CreateJourney(ctx, journey.Journey{
		ProjectID:   project.ID,
		Name:        body.Name,
		Description: body.Description,
	})
	if err != nil {
		logger.Error("failed to create journey", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	versionID, err := journeys.CreateJourneyVersion(ctx, journeyID, "draft")
	if err != nil {
		logger.Error("failed to create initial version", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	err = journeys.UpdateJourney(ctx, projectID, journeyID, journey.JourneyUpdate{VersionID: &versionID})
	if err != nil {
		logger.Error("failed to link journey to version", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("journey created with initial version",
		zap.Stringer("journey_id", journeyID),
		zap.Stringer("version_id", versionID))

	if params.Publish != nil && *params.Publish {
		err = journeys.PublishVersion(ctx, journeyID, versionID)
		if err != nil {
			logger.Error("failed to publish journey", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
		logger.Info("journey published immediately", zap.Stringer("version_id", versionID))
	}

	journey, err := journeys.GetJourney(ctx, projectID, journeyID)
	if err != nil {
		logger.Error("failed to get journey", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	versionInfo, err := journeys.GetJourneyVersionInfo(ctx, journeyID)
	if err != nil {
		logger.Error("failed to get journey version info", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	err = tx.Commit()
	if err != nil {
		logger.Error("failed to commit transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	json.Write(w, http.StatusCreated, journey.OAPI(versionInfo))
}

func (srv *JourneysController) GetJourney(w http.ResponseWriter, r *http.Request, projectID, journeyID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.Stringer("journey_id", journeyID),
	)

	logger.Info("getting journey")

	jrn, err := srv.jrny.GetJourney(ctx, projectID, journeyID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("journey not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("journey not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get journey", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	versionInfo, err := srv.jrny.GetJourneyVersionInfo(ctx, journeyID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		logger.Error("failed to get version info", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("journey retrieved")
	json.Write(w, http.StatusOK, jrn.OAPI(versionInfo))
}

func (srv *JourneysController) UpdateJourney(w http.ResponseWriter, r *http.Request, projectID, journeyID uuid.UUID) {
	ctx := r.Context()
	body := oapi.UpdateJourneyJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.Stringer("journey_id", journeyID),
	)

	logger.Info("updating journey")

	_, err = srv.jrny.GetJourney(ctx, projectID, journeyID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("journey not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("journey not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get journey", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	tx, err := srv.journeyDB.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	defer tx.Rollback() //nolint:errcheck

	journeys := journey.NewJourneysStore(tx)

	updated := journey.JourneyUpdate{
		Name:        body.Name,
		Description: body.Description,
	}

	err = journeys.UpdateJourney(ctx, projectID, journeyID, updated)
	if err != nil {
		logger.Error("failed to update journey", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	journey, err := journeys.GetJourney(ctx, projectID, journeyID)
	if err != nil {
		logger.Error("failed to get journey", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	err = tx.Commit()
	if err != nil {
		logger.Error("failed to commit transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	versionInfo, err := srv.jrny.GetJourneyVersionInfo(ctx, journeyID)
	if err != nil {
		logger.Error("failed to get journey version info", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("journey updated")
	json.Write(w, http.StatusOK, journey.OAPI(versionInfo))
}

func (srv *JourneysController) DeleteJourney(w http.ResponseWriter, r *http.Request, projectID, journeyID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.Stringer("journey_id", journeyID),
	)

	logger.Info("deleting journey")

	_, err := srv.jrny.GetJourney(ctx, projectID, journeyID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("journey not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("journey not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get journey", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	err = srv.jrny.DeleteJourney(ctx, projectID, journeyID)
	if err != nil {
		logger.Error("failed to delete journey", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("journey deleted")
	w.WriteHeader(http.StatusNoContent)
}

func (srv *JourneysController) GetJourneySteps(w http.ResponseWriter, r *http.Request, projectID, journeyID uuid.UUID) {
	ctx := r.Context()

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("journey_id", journeyID))
	logger.Info("getting journey steps")

	_, err := srv.jrny.GetJourney(ctx, projectID, journeyID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("journey not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("journey not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get journey", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	versionID, err := srv.jrny.ResolveVersionID(ctx, journeyID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("no version exists, returning empty step map")
		json.Write(w, http.StatusOK, oapi.JourneyStepMap{})
		return
	}
	if err != nil {
		logger.Error("failed to resolve version", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	steps, err := srv.jrny.GetJourneyVersionSteps(ctx, versionID)
	if err != nil {
		logger.Error("failed to get journey steps with children", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("journey steps retrieved")
	json.Write(w, http.StatusOK, steps.OAPI())
}

func (srv *JourneysController) SetJourneySteps(w http.ResponseWriter, r *http.Request, projectID, journeyID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.Stringer("journey_id", journeyID),
	)

	logger.Info("setting journey steps")

	_, err := srv.jrny.GetJourney(ctx, projectID, journeyID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("journey not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("journey not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get journey", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	var steps oapi.JourneyStepMap
	err = json.Decode(r.Body, &steps)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	tx, err := srv.journeyDB.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	defer tx.Rollback() //nolint:errcheck

	journeys := journey.NewJourneysStore(tx)
	events := subjects.NewEventsStore(srv.subjectsDB)

	versionID, err := journeys.EnsureDraftVersion(ctx, journeyID)
	if err != nil {
		logger.Error("failed to ensure draft version", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	stepIDs, err := journeys.SetJourneySteps(ctx, versionID, steps)
	if err != nil {
		logger.Error("failed to set journey steps", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	dependencies, err := journeyEntranceEventDependencies(steps)
	if err != nil {
		logger.Error("failed to get entrance event dependencies", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	for externalID, eventName := range dependencies {
		eventID, err := events.UpsertEvent(ctx, projectID, eventName, subjects.SubjectTypeUser)
		if err != nil {
			logger.Error("failed to upsert event", zap.String("event", eventName), zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}

		err = journeys.SetJourneyStepEventDependencies(ctx, versionID, externalID, []uuid.UUID{eventID})
		if err != nil {
			logger.Error("failed to set journey step event dependencies", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		logger.Error("failed to commit transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("journey steps set", zap.Int("step_count", len(stepIDs)))
	srv.GetJourneySteps(w, r, projectID, journeyID)
}

func (srv *JourneysController) VersionJourney(w http.ResponseWriter, r *http.Request, projectID, journeyID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.Stringer("journey_id", journeyID),
	)

	logger.Info("creating journey draft version")

	_, err := srv.jrny.GetJourney(ctx, projectID, journeyID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("journey not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("journey not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get journey", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	tx, err := srv.journeyDB.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	defer tx.Rollback() //nolint:errcheck

	journeys := journey.NewJourneysStore(tx)

	// Create or get existing draft version
	draftVersionID, err := journeys.EnsureDraftVersion(ctx, journeyID)
	if err != nil {
		logger.Error("failed to ensure draft version", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Get the updated journey
	result, err := journeys.GetJourney(ctx, projectID, journeyID)
	if err != nil {
		logger.Error("failed to get journey", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	err = tx.Commit()
	if err != nil {
		logger.Error("failed to commit transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	versionInfo, err := srv.jrny.GetJourneyVersionInfo(ctx, journeyID)
	if err != nil {
		logger.Error("failed to get journey version info", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("journey draft version created", zap.Stringer("draft_version_id", draftVersionID))
	json.Write(w, http.StatusCreated, result.OAPI(versionInfo))
}

func (srv *JourneysController) DuplicateJourney(w http.ResponseWriter, r *http.Request, projectID, journeyID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.Stringer("journey_id", journeyID),
	)

	logger.Info("duplicating journey")

	jrn, err := srv.jrny.GetJourney(ctx, projectID, journeyID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("journey not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("journey not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get journey", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	tx, err := srv.journeyDB.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	defer tx.Rollback() //nolint:errcheck

	journeys := journey.NewJourneysStore(tx)

	duplicateJourneyID, err := journeys.DuplicateJourney(ctx, jrn, false)
	if err != nil {
		logger.Error("failed to duplicate journey", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	result, err := journeys.GetJourney(ctx, projectID, duplicateJourneyID)
	if err != nil {
		logger.Error("failed to get new journey", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	err = tx.Commit()
	if err != nil {
		logger.Error("failed to commit transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	versionInfo, err := srv.jrny.GetJourneyVersionInfo(ctx, duplicateJourneyID)
	if err != nil {
		logger.Error("failed to get journey version info", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("journey duplicated", zap.Stringer("journey_id", duplicateJourneyID))
	json.Write(w, http.StatusCreated, result.OAPI(versionInfo))
}

func (srv *JourneysController) PublishJourney(w http.ResponseWriter, r *http.Request, projectID, journeyID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.Stringer("journey_id", journeyID),
	)

	logger.Info("publishing journey")

	_, err := srv.jrny.GetJourney(ctx, projectID, journeyID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("journey not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("journey not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get journey", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Get latest draft version
	draftVersion, err := srv.jrny.GetLatestDraftVersion(ctx, journeyID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("no draft version found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("no draft version found to publish")))
		return
	}

	if err != nil {
		logger.Error("failed to get draft version", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	tx, err := srv.journeyDB.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	defer tx.Rollback() //nolint:errcheck

	journeys := journey.NewJourneysStore(tx)

	err = journeys.PublishVersion(ctx, journeyID, draftVersion.ID)
	if err != nil {
		logger.Error("failed to publish version", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	updated, err := journeys.GetJourney(ctx, projectID, journeyID)
	if err != nil {
		logger.Error("failed to get updated journey", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	versionInfo, err := journeys.GetJourneyVersionInfo(ctx, journeyID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		logger.Error("failed to get version info", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	err = tx.Commit()
	if err != nil {
		logger.Error("failed to commit transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("journey published", zap.Stringer("version_id", draftVersion.ID))
	json.Write(w, http.StatusOK, updated.OAPI(versionInfo))
}

// journeyEntranceEventDependencies extracts event dependencies from entrance steps
func journeyEntranceEventDependencies(steps oapi.JourneyStepMap) (map[string]string, error) {
	events := make(map[string]string)
	for id, step := range steps {
		if step.Type != oapi.JourneyStepTypeEntrance {
			continue
		}

		var data oapi.EntranceStepData
		err := json.Unmarshal(step.Data, &data)
		if err != nil {
			return nil, err
		}

		if data.Trigger != nil && *data.Trigger == "event" && data.EventName != nil {
			events[id] = *data.EventName
		}
	}

	return events, nil
}
