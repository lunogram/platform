package v1

import (
	stdjson "encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/actions"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

func NewActionsController(logger *zap.Logger, db *sqlx.DB, req pubsub.Caller, usersDB *sqlx.DB, actionRegistry *actions.Registry) *ActionsController {
	return &ActionsController{
		logger:   logger,
		db:       db,
		store:    management.NewState(db),
		registry: actionRegistry,
		req:      req,
		subjects: subjects.NewState(usersDB),
	}
}

type ActionsController struct {
	logger   *zap.Logger
	db       *sqlx.DB
	store    *management.State
	registry *actions.Registry
	req      pubsub.Caller
	subjects *subjects.State
}

func (srv *ActionsController) CreateAction(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	body := oapi.CreateActionJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("name", body.Name))
	logger.Info("creating action")

	// Validate action type against registered modules
	if _, exists := srv.registry.Get(string(body.Type)); !exists {
		logger.Warn("unknown action type", zap.String("type", string(body.Type)))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("unknown action type")))
		return
	}

	var config stdjson.RawMessage
	if body.Config != nil {
		config = *body.Config
	}

	action, err := srv.store.ActionsStore.CreateAction(ctx, projectID, body.Name, string(body.Type), config)
	if err != nil {
		logger.Error("failed to create action", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("action created", zap.Stringer("action_id", action.ID))
	json.Write(w, http.StatusCreated, action.OAPI())
}

func (srv *ActionsController) ListActions(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListActionsParams) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("listing actions")

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	result, total, err := srv.store.ActionsStore.ListActions(ctx, projectID, pagination, params.Search.ToString())
	if err != nil {
		logger.Error("failed to list actions", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("listed actions", zap.Int("count", len(result)))
	json.Write(w, http.StatusOK, oapi.ActionListResponse{
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		Results: result.OAPI(),
	})
}

func (srv *ActionsController) GetAction(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, actionID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("action_id", actionID))
	logger.Info("getting action")

	action, err := srv.store.ActionsStore.GetAction(ctx, projectID, actionID)
	if errors.Is(err, store.ErrNoRows) {
		logger.Info("action not found", zap.Stringer("action_id", actionID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("Action not found")))
		return
	}

	if err != nil {
		logger.Error("failed to fetch action", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("action retrieved")
	json.Write(w, http.StatusOK, action.OAPI())
}

func (srv *ActionsController) UpdateAction(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, actionID uuid.UUID) {
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("action_id", actionID))
	logger.Info("updating action")

	ctx := r.Context()
	body := oapi.UpdateActionJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var actionType *string
	if body.Type != nil {
		// Validate action type against registered modules
		if _, exists := srv.registry.Get(string(*body.Type)); !exists {
			logger.Warn("unknown action type", zap.String("type", string(*body.Type)))
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("unknown action type")))
			return
		}
		t := string(*body.Type)
		actionType = &t
	}

	var config stdjson.RawMessage
	if body.Config != nil {
		config = *body.Config
	}

	err = srv.store.ActionsStore.UpdateAction(ctx, projectID, actionID, body.Name, actionType, config)
	if err != nil {
		logger.Error("failed to update action", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	action, err := srv.store.ActionsStore.GetAction(ctx, projectID, actionID)
	if errors.Is(err, store.ErrNoRows) {
		logger.Info("action not found", zap.Stringer("action_id", actionID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("Action not found")))
		return
	}

	if err != nil {
		logger.Error("failed to fetch updated action", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("action updated")
	json.Write(w, http.StatusOK, action.OAPI())
}

func (srv *ActionsController) DeleteAction(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, actionID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("action_id", actionID))
	logger.Info("deleting action")

	err := srv.store.ActionsStore.DeleteAction(ctx, projectID, actionID)
	if err != nil {
		logger.Error("failed to delete action", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("action deleted")
	w.WriteHeader(http.StatusNoContent)
}

func (srv *ActionsController) ListActionMeta(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("listing action meta")

	allActions := srv.registry.All()
	meta := make([]oapi.ActionMeta, 0, len(allActions))

	for _, a := range allActions {
		manifest := a.Manifest()

		// Skip hidden modules from the UI listing
		if manifest.Metadata.Hidden {
			continue
		}

		m := oapi.ActionMeta{
			Type:        manifest.Metadata.ID,
			Name:        manifest.Metadata.Title,
			Description: &manifest.Metadata.Description,
		}

		if manifest.Spec.Config != nil {
			schema, err := json.Marshal(manifest.Spec.Config)
			if err != nil {
				logger.Error("failed to marshal config schema", zap.String("module", manifest.Metadata.ID), zap.Error(err))
				oapi.WriteProblem(w, err)
				return
			}
			raw := json.RawMessage(schema)
			m.ConfigSchema = &raw
		}

		if manifest.Spec.Payload != nil {
			schema, err := json.Marshal(manifest.Spec.Payload)
			if err != nil {
				logger.Error("failed to marshal payload schema", zap.String("module", manifest.Metadata.ID), zap.Error(err))
				oapi.WriteProblem(w, err)
				return
			}
			raw := json.RawMessage(schema)
			m.PayloadSchema = &raw
		}

		meta = append(meta, m)
	}

	logger.Info("listed action meta", zap.Int("count", len(meta)))
	json.Write(w, http.StatusOK, meta)
}

func (srv *ActionsController) GetActionPreview(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, actionType string) {
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("action_type", actionType))
	logger.Info("getting action preview")

	action, exists := srv.registry.Get(actionType)
	if !exists {
		logger.Info("action module not found", zap.String("action_type", actionType))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("action module not found")))
		return
	}

	preview, err := action.Preview(r.Context())
	if err != nil {
		logger.Error("failed to get action preview", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(preview) //nolint:errcheck
}

func (srv *ActionsController) TestAction(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID))

	body := oapi.TestActionJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger = logger.With(zap.String("action_type", string(body.Type)))
	logger.Info("testing action")

	// Validate the action type exists before publishing.
	if _, exists := srv.registry.Get(string(body.Type)); !exists {
		logger.Warn("unknown action type")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("unknown action type")))
		return
	}

	// Build variables map from raw JSON.
	var variables map[string]any
	if body.Variables != nil {
		if err := stdjson.Unmarshal(*body.Variables, &variables); err != nil {
			logger.Error("failed to unmarshal variables", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid variables")))
			return
		}
	}

	// Build config from request body (allows testing unsaved actions).
	var config map[string]any
	if body.Config != nil {
		if err := stdjson.Unmarshal(*body.Config, &config); err != nil {
			logger.Error("failed to unmarshal config", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid config")))
			return
		}
	}
	if config == nil {
		config = map[string]any{}
	}

	// Resolve optional action ID.
	var actionID uuid.UUID
	if body.ActionId != nil {
		actionID = uuid.UUID(*body.ActionId)
	}

	// Publish the action execution request via NATS and wait for reply.
	executeReq := schemas.ExecuteAction{
		ProjectID: projectID,
		ActionID:  actionID,
		Type:      string(body.Type),
		Config:    config,
		Payload:   body.Payload,
		Variables: variables,
	}

	data, err := srv.req.Call(ctx, schemas.ActionsExecute(projectID), executeReq, 30*time.Second)
	if err != nil {
		logger.Error("action execution request failed", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	var result schemas.ExecuteActionResponse
	if err := stdjson.Unmarshal(data, &result); err != nil {
		logger.Error("failed to unmarshal action response", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	resp := oapi.TestActionResult{
		Status:     result.Status,
		StatusCode: result.StatusCode,
	}

	if result.Error != "" {
		resp.Error = &result.Error
	}

	if result.Metadata != nil {
		resp.Metadata = &result.Metadata
	}

	logger.Info("action test completed", zap.String("status", result.Status))
	json.Write(w, http.StatusOK, resp)
}

func (srv *ActionsController) ListActionSchemas(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, actionID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("action_id", actionID))
	logger.Info("listing action schemas")

	paths, err := srv.subjects.ListActionSchemas(ctx, actionID)
	if err != nil {
		logger.Error("failed to list action schemas", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	results := make([]oapi.SchemaPath, len(paths))
	for i, p := range paths {
		results[i] = oapi.SchemaPath{
			Path:  p.Path,
			Types: []string(p.Types),
		}
	}

	logger.Info("action schemas listed", zap.Int("count", len(results)))
	json.Write(w, http.StatusOK, oapi.ActionSchemaListResponse{
		Results: results,
	})
}
