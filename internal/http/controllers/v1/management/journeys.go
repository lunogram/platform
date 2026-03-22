package v1

import (
	"database/sql"
	stdjson "encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/http/sse"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/consumer"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

func NewJourneysController(logger *zap.Logger, journeyDB, subjectsDB *sqlx.DB, mgmt *management.State, pub pubsub.Publisher, jet jetstream.JetStream, engine *rbac.Engine) *JourneysController {
	return &JourneysController{
		logger:     logger,
		journeyDB:  journeyDB,
		subjectsDB: subjectsDB,
		mgmt:       mgmt,
		jrny:       journey.NewState(journeyDB),
		subjects:   subjects.NewState(subjectsDB, logger),
		pub:        pub,
		jet:        jet,
		engine:     engine,
	}
}

type JourneysController struct {
	logger     *zap.Logger
	journeyDB  *sqlx.DB
	subjectsDB *sqlx.DB
	mgmt       *management.State
	subjects   *subjects.State
	jrny       *journey.State
	pub        pubsub.Publisher
	jet        jetstream.JetStream
	engine     *rbac.Engine
}

func (srv *JourneysController) ListJourneys(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListJourneysParams) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("journeys", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}
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
	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("journeys", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}
	body := oapi.CreateJourneyJSONRequestBody{}
	err = json.Decode(r.Body, &body)
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
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("journeys", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}
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
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("journeys", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}
	body := oapi.UpdateJourneyJSONRequestBody{}
	err = json.Decode(r.Body, &body)
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
	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("journeys", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}
	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.Stringer("journey_id", journeyID),
	)

	logger.Info("deleting journey")

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

	err = srv.jrny.DeleteJourney(ctx, projectID, journeyID)
	if err != nil {
		logger.Error("failed to delete journey", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("journey deleted")
	w.WriteHeader(http.StatusNoContent)
}

func (srv *JourneysController) StreamUserJourneySteps(
	w http.ResponseWriter,
	r *http.Request,
	projectID uuid.UUID,
	journeyID uuid.UUID,
	userID uuid.UUID,
) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("journeys", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}
	enc := sse.NewEncoder(w)
	enc.WriteEvent("message", "connected")
	rc := http.NewResponseController(w)

	err = rc.SetWriteDeadline(time.Time{})
	if err != nil {
		srv.logger.Error("failed to set write deadline", zap.Error(err))
		return
	}

	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.Stringer("journey_id", journeyID),
		zap.Stringer("user_id", userID),
	)
	logger.Info("streaming user journey steps")

	nconn := srv.jet.Conn()
	journeyAdvanceSubject := schemas.JourneysAdvance(projectID, journeyID, userID)
	journeyAdvanceSub, err := nconn.Subscribe(string(journeyAdvanceSubject), func(msg *nats.Msg) {
		enc.WriteEvent("step", json.RawMessage(msg.Data))
	})

	if err != nil {
		enc.WriteHeader(http.StatusInternalServerError)
		logger.Error("subscribe failed", zap.Error(err))
		return
	}

	//nolint: errcheck
	defer journeyAdvanceSub.Unsubscribe()

	executedSubject := schemas.JourneysStepExecuted(projectID, journeyID, userID)
	executedSub, err := nconn.Subscribe(string(executedSubject), func(msg *nats.Msg) {
		enc.WriteEvent("step_executed", json.RawMessage(msg.Data))
	})

	if err != nil {
		enc.WriteHeader(http.StatusInternalServerError)
		logger.Error("subscribe to step executed failed", zap.Error(err))
		return
	}

	//nolint: errcheck
	defer executedSub.Unsubscribe()

	<-ctx.Done()
	logger.Info("client disconnected")
}

func (srv *JourneysController) GetUserJourneyState(w http.ResponseWriter, r *http.Request, projectID, journeyID, userID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("journeys", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.Stringer("journey_id", journeyID),
		zap.Stringer("user_id", userID),
	)
	logger.Info("getting user journey state")

	states, err := srv.jrny.GetUserJourneyCurrentState(ctx, projectID, journeyID, userID)
	if err != nil {
		logger.Error("failed to get user journey state", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	type responseItem struct {
		ExternalStepID string `json:"external_step_id"`
		StepType       string `json:"step_type"`
		IsCompleted    bool   `json:"is_completed"`
	}

	response := make([]responseItem, len(states))
	for i, state := range states {
		response[i] = responseItem{
			ExternalStepID: state.ExternalStepID,
			StepType:       state.StepType,
			IsCompleted:    state.CompletedAt != nil,
		}
	}

	json.Write(w, http.StatusOK, response)
}

func (srv *JourneysController) TriggerUser(w http.ResponseWriter, r *http.Request, projectID, journeyID uuid.UUID, userID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("journeys", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.TriggerUserJSONRequestBody{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	externalStepIDStr := body.ExternalStepID.String()

	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.Stringer("journey_id", journeyID),
		zap.Stringer("user_id", userID),
		zap.String("external_step_id", externalStepIDStr),
	)

	logger.Info("triggering user into journey")

	journeys := journey.NewJourneysStore(srv.journeyDB)

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

	if currJourney.VersionID == nil {
		logger.Info("no published version for journey, returning error")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("journey has no published version")))
		return
	}

	journeyStep, err := journeys.GetJourneyStep(ctx, journeyID, externalStepIDStr, currJourney.VersionID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("journey step not found", zap.String("external_step_id", externalStepIDStr))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("journey step not found")))
		return
	}

	state := journey.JourneyUserState{
		UserID:          userID,
		JourneyID:       journeyID,
		JourneyEntryID:  journeyStep.ID,
		ExternalStepID:  externalStepIDStr,
		PinnedVersionID: currJourney.VersionID,
	}

	if body.Data != nil {
		wrapped := map[string]any{
			"data": body.Data,
		}
		dataBytes, err := stdjson.Marshal(wrapped)
		if err != nil {
			logger.Error("failed to marshal trigger data", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
		state.Data = dataBytes
	}

	_, err = journeys.CreateUserJourneyState(ctx, state)

	if err != nil {
		logger.Error("failed to create user journey state", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	for _, child := range journeyStep.Children {
		stepType, err := journeys.GetStepType(ctx, *currJourney.VersionID, child.ChildExternalID)
		if err != nil {
			logger.Error("failed to get step type for child", zap.Error(err))
			continue
		}

		step := consumer.JourneyStep{
			ProjectID:      currJourney.ProjectID,
			JourneyID:      journeyID,
			JourneyEntryID: journeyStep.ID,
			ExternalStepID: child.ChildExternalID,
			UserID:         userID,
			VersionID:      currJourney.VersionID,
			StepType:       stepType,
		}

		err = srv.pub.Publish(ctx, schemas.JourneysAdvance(currJourney.ProjectID, step.JourneyID, step.UserID), step)
		if err != nil {
			logger.Error("failed to publish journey state to project subject", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
	}

	logger.Info("journey triggered for user")
	json.Write(w, http.StatusOK, "Journey triggered successfully for user")
}

func (srv *JourneysController) AdvanceUserStep(w http.ResponseWriter, r *http.Request, projectID, journeyID, userID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("journeys", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.AdvanceUserStepJSONRequestBody{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	externalStepIDStr := body.ExternalStepID.String()

	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.Stringer("journey_id", journeyID),
		zap.Stringer("user_id", userID),
		zap.String("external_step_id", externalStepIDStr),
	)
	logger.Info("skipping user journey step")

	state, err := srv.jrny.ResumeUserJourneyStep(ctx, journeyID, userID, externalStepIDStr)
	if err != nil {
		logger.Error("failed to skip user journey step", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	stepType, err := srv.jrny.GetStepType(ctx, *state.PinnedVersionID, state.ExternalStepID)
	if err != nil {
		logger.Error("failed to get step type", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	step := schemas.JourneyStep{
		ProjectID:      projectID,
		JourneyID:      state.JourneyID,
		JourneyEntryID: state.JourneyEntryID,
		VersionID:      state.PinnedVersionID,
		UserID:         state.UserID,
		ExternalStepID: state.ExternalStepID,
		StepType:       stepType,
		StateID:        &state.ID,
	}

	err = srv.pub.Publish(ctx, schemas.JourneysAdvance(projectID, state.JourneyID, state.UserID), step)
	if err != nil {
		logger.Error("failed to publish skipped journey step", zap.Error(err), zap.Stringer("state_id", state.ID))
		oapi.WriteProblem(w, err)
		return
	}

	json.Write(w, http.StatusOK, "User journey step skipped")
}

func (srv *JourneysController) GetJourneySteps(w http.ResponseWriter, r *http.Request, projectID, journeyID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("journeys", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("journey_id", journeyID))
	logger.Info("getting journey steps")

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
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("journeys", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}
	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.Stringer("journey_id", journeyID),
	)

	logger.Info("setting journey steps")

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

	eventDependencies, err := journeyEntranceEventDependencies(steps)
	if err != nil {
		logger.Error("failed to get entrance event dependencies", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	for externalID, eventName := range eventDependencies {
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
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("journeys", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}
	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.Stringer("journey_id", journeyID),
	)

	logger.Info("creating journey draft version")

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
	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("journeys", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}
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
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("journeys", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}
	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.Stringer("journey_id", journeyID),
	)

	logger.Info("publishing journey")

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

		if data.Trigger != nil && (*data.Trigger == "event" || *data.Trigger == "scheduled") && data.EventName != nil {
			events[id] = *data.EventName
		}
	}

	return events, nil
}
