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
	"github.com/lunogram/platform/internal/http/sse"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/consumer"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/users"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

func NewJourneysController(logger *zap.Logger, journeyDB *sqlx.DB, mgmt *management.State, pub pubsub.Publisher, jet jetstream.JetStream) *JourneysController {
	return &JourneysController{
		logger: logger,
		db:     journeyDB,
		mgmt:   mgmt,
		jrny:   journey.NewState(journeyDB),
		users:  users.NewState(journeyDB),
		pub:    pub,
		jet:    jet,
	}
}

type JourneysController struct {
	logger *zap.Logger
	db     *sqlx.DB
	mgmt   *management.State
	users  *users.State
	jrny   *journey.State
	pub    pubsub.Publisher
	jet    jetstream.JetStream
}

func (srv *JourneysController) ListJourneys(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListJourneysParams) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID))

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	logger.Info("listing journeys", zap.Int("limit", pagination.Limit), zap.Int("offset", pagination.Offset))

	journeys, total, err := srv.jrny.ListJourneys(ctx, projectID, pagination)
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

func (srv *JourneysController) CreateJourney(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
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

	tx, err := srv.db.BeginTxx(ctx, nil)
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

	tx, err := srv.db.BeginTxx(ctx, nil)
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

func (srv *JourneysController) FollowUserJourneyStatus(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, journeyID uuid.UUID, params oapi.FollowUserJourneyStatusParams) {
	ctx := r.Context()
	enc := sse.NewEncoder(w)

	userID := params.UserID

	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.Stringer("journey_id", journeyID),
		zap.Stringer("user_id", userID),
	)

	logger.Info("following user journey status")

	nconn := srv.jet.Conn()

	handler := func(msg *nats.Msg) {
		if ctx.Done() != nil {
			logger.Info("stopping user journey status follow")
			return
		}

		enc.WriteEvent("UserAdvanced", msg.Data)
	}

	subject := schemas.JourneysAdvance(projectID, journeyID, userID)
	_, err := nconn.Subscribe(string(subject), handler)
	if err != nil {
		logger.Error("failed to subscribe to journey updates", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
}

func (srv *JourneysController) RunJourneyForUser(w http.ResponseWriter, r *http.Request, projectID, journeyID uuid.UUID) {
	ctx := r.Context()

	body := oapi.RunJourneyForUserJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	userIDStr := body.UserID
	externalStepIDStr := body.ExternalStepID

	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.Stringer("journey_id", journeyID),
		zap.String("user_id", userIDStr),
		zap.String("external_step_id", externalStepIDStr),
	)

	logger.Info("running journey for user")

	journeys := journey.NewJourneysStore(srv.db)

	currJourney, err := journeys.GetJourney(ctx, projectID, journeyID)
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

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		logger.Error("invalid user_id format", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid user_id format")))
		return
	}

	_, err = journeys.CreateUserJourneyState(ctx, journey.JourneyUserState{
		UserID:          userID,
		JourneyID:       journeyID,
		ExternalStepID:  externalStepIDStr,
		PinnedVersionID: currJourney.VersionID,
	})

	if err != nil {
		logger.Error("failed to create user journey state", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if currJourney.VersionID == nil {
		logger.Info("no published version for journey, returning error")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("journey has no published version")))
		return
	}

	step, err := srv.users.GetEventJourneyStep(ctx, externalStepIDStr, *currJourney.VersionID)
	if err != nil {
		logger.Error("failed to get event journey step dependencies", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	for _, child := range step.Children {
		step := consumer.JourneyStep{
			ProjectID:      currJourney.ProjectID,
			JourneyID:      step.JourneyID,
			ExternalStepID: child.ChildExternalID,
			UserID:         userID,
		}

		err = srv.pub.Publish(ctx, schemas.JourneysAdvance(currJourney.ProjectID, step.JourneyID, step.UserID), step)
		if err != nil {
			logger.Error("failed to publish journey state to project subject", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
	}

	logger.Info("journey run for user")
	json.Write(w, http.StatusOK, "Journey run successfully for user")
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

	tx, err := srv.db.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	defer tx.Rollback() //nolint:errcheck

	journeys := journey.NewJourneysStore(tx)
	events := users.NewEventsStore(tx)

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
		eventID, err := events.UpsertEvent(ctx, projectID, eventName)
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

	tx, err := srv.db.BeginTxx(ctx, nil)
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

	tx, err := srv.db.BeginTxx(ctx, nil)
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

	tx, err := srv.db.BeginTxx(ctx, nil)
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
