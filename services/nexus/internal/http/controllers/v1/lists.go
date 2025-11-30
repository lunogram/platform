package v1

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	pkgjson "github.com/lunogram/platform/pkg/http/json"
	"github.com/lunogram/platform/pkg/http/problem"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/lunogram/platform/services/nexus/oapi"
	"go.uber.org/zap"
)

func NewListsController(logger *zap.Logger, db *sqlx.DB) *ListsController {
	return &ListsController{
		logger: logger,
		db:     db,
		store:  store.NewStores(db),
	}
}

type ListsController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *store.Stores
}

func (srv *ListsController) CreateList(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	body := oapi.CreateListJSONRequestBody{}
	err := pkgjson.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("name", body.Name))
	logger.Info("creating list")

	_, err = srv.store.ProjectsStore.GetProject(ctx, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("project not found", zap.Stringer("project_id", projectID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("project not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get project", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	var rule store.RuleData
	if body.Rule != nil {
		if err := json.Unmarshal(*body.Rule, &rule); err != nil {
			logger.Error("failed to unmarshal rule", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid rule format")))
			return
		}
	}

	// Static lists start in 'ready' state since they don't require rule configuration.
	// Dynamic lists start in 'draft' state to allow rule setup before activation.
	state := "ready"
	if body.Type == oapi.CreateListTypeDynamic {
		state = "draft"
	}

	isVisible := true
	if body.IsVisible != nil {
		isVisible = *body.IsVisible
	}

	usersCount := 0

	listID, err := srv.store.CreateList(ctx, store.List{
		ProjectID:  projectID,
		Name:       body.Name,
		Type:       string(body.Type),
		State:      state,
		Rule:       store.JSONB[store.RuleData]{Data: rule},
		IsVisible:  isVisible,
		UsersCount: &usersCount,
		Version:    0,
	})
	if err != nil {
		logger.Error("failed to create list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("list created", zap.Stringer("list_id", listID))
	list, err := srv.store.GetList(ctx, projectID, listID)
	if err != nil {
		logger.Error("failed to fetch created list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	pkgjson.Write(w, http.StatusCreated, list.OAPI())
}

func (srv *ListsController) ListLists(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListListsParams) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("listing lists")

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	result, total, err := srv.store.ListLists(ctx, projectID, pagination)
	if err != nil {
		logger.Error("failed to list lists", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("listed lists", zap.Int("count", len(result)))
	pkgjson.Write(w, http.StatusOK, oapi.ListListResponse{
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		Results: result.OAPI(),
	})
}

func (srv *ListsController) GetList(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, listID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("list_id", listID))
	logger.Info("getting list")

	list, err := srv.store.GetList(ctx, projectID, listID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("list not found", zap.Stringer("list_id", listID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("list not found")))
		return
	}

	if err != nil {
		logger.Error("failed to fetch list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("list retrieved")
	pkgjson.Write(w, http.StatusOK, list.OAPI())
}

func (srv *ListsController) UpdateList(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, listID uuid.UUID) {
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("list_id", listID))
	logger.Info("updating list")

	ctx := r.Context()
	body := oapi.UpdateListJSONRequestBody{}
	err := pkgjson.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.store.GetList(ctx, projectID, listID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("list not found", zap.Stringer("list_id", listID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("list not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	update := store.ListUpdate{
		Name:      &body.Name,
		Published: body.Published,
	}

	if body.Rule != nil {
		var rule store.RuleData
		if err := json.Unmarshal(*body.Rule, &rule); err != nil {
			logger.Error("failed to unmarshal rule", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid rule format")))
			return
		}
		update.Rule = &store.JSONB[store.RuleData]{Data: rule}
	}

	err = srv.store.UpdateList(ctx, projectID, listID, update)
	if err != nil {
		logger.Error("failed to update list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	list, err := srv.store.GetList(ctx, projectID, listID)
	if err != nil {
		logger.Error("failed to fetch updated list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("list updated")
	pkgjson.Write(w, http.StatusOK, list.OAPI())
}

func (srv *ListsController) DeleteList(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, listID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("list_id", listID))
	logger.Info("deleting list")

	_, err := srv.store.GetList(ctx, projectID, listID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("list not found", zap.Stringer("list_id", listID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("list not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	err = srv.store.DeleteList(ctx, projectID, listID)
	if err != nil {
		logger.Error("failed to delete list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("list deleted")
	w.WriteHeader(http.StatusNoContent)
}

func (srv *ListsController) DuplicateList(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, listID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("list_id", listID))
	logger.Info("duplicating list")

	list, err := srv.store.GetList(ctx, projectID, listID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("list not found", zap.Stringer("list_id", listID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("list not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	newName := "Copy of " + list.Name
	newListID, err := srv.store.DuplicateList(ctx, projectID, listID, newName)
	if err != nil {
		logger.Error("failed to duplicate list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("list duplicated", zap.Stringer("new_list_id", newListID))
	duplicated, err := srv.store.GetList(ctx, projectID, newListID)
	if err != nil {
		logger.Error("failed to fetch duplicated list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	pkgjson.Write(w, http.StatusCreated, duplicated.OAPI())
}
