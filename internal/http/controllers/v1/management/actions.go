package v1

import (
	stdjson "encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

func NewActionsController(logger *zap.Logger, db *sqlx.DB) *ActionsController {
	return &ActionsController{
		logger: logger,
		db:     db,
		store:  management.NewState(db),
	}
}

type ActionsController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *management.State
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
