package v1

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

func NewTagsController(logger *zap.Logger, db *sqlx.DB, engine *rbac.Engine) *TagsController {
	return &TagsController{
		logger: logger,
		db:     db,
		store:  management.NewState(db),
		engine: engine,
	}
}

type TagsController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *management.State
	engine *rbac.Engine
}

func (srv *TagsController) CreateTag(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("tags", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.CreateTagJSONRequestBody{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("name", body.Name))
	logger.Info("creating tag")

	tagID, err := srv.store.TagsStore.CreateTag(ctx, projectID, body.Name)
	if err != nil {
		logger.Error("failed to create tag", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	tag, err := srv.store.TagsStore.GetTag(ctx, projectID, tagID)
	if err != nil {
		logger.Error("failed to fetch created tag", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("tag created", zap.Stringer("tag_id", tagID))
	json.Write(w, http.StatusCreated, tag.OAPI())
}

func (srv *TagsController) ListTags(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListTagsParams) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("tags", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("listing tags")

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	result, total, err := srv.store.TagsStore.ListTags(ctx, projectID, pagination, params.Search.ToString())
	if err != nil {
		logger.Error("failed to list tags", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("listed tags", zap.Int("count", len(result)))
	json.Write(w, http.StatusOK, oapi.TagListResponse{
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		Results: result.OAPI(),
	})
}

func (srv *TagsController) GetTag(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, tagID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("tags", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("tag_id", tagID))
	logger.Info("getting tag")

	tag, err := srv.store.TagsStore.GetTag(ctx, projectID, tagID)
	if errors.Is(err, store.ErrNoRows) {
		logger.Info("tag not found", zap.Stringer("tag_id", tagID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("tag not found")))
		return
	}

	if err != nil {
		logger.Error("failed to fetch tag", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("tag retrieved")
	json.Write(w, http.StatusOK, tag.OAPI())
}

func (srv *TagsController) UpdateTag(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, tagID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("tags", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("tag_id", tagID))
	logger.Info("updating tag")

	body := oapi.UpdateTagJSONRequestBody{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	err = srv.store.TagsStore.UpdateTag(ctx, projectID, tagID, body.Name)
	if err != nil {
		logger.Error("failed to update tag", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	tag, err := srv.store.TagsStore.GetTag(ctx, projectID, tagID)
	if errors.Is(err, store.ErrNoRows) {
		logger.Info("tag not found", zap.Stringer("tag_id", tagID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("tag not found")))
		return
	}

	if err != nil {
		logger.Error("failed to fetch updated tag", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("tag updated")
	json.Write(w, http.StatusOK, tag.OAPI())
}

func (srv *TagsController) DeleteTag(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, tagID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("tags", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("tag_id", tagID))
	logger.Info("deleting tag")

	err = srv.store.TagsStore.DeleteTag(ctx, projectID, tagID)
	if err != nil {
		logger.Error("failed to delete tag", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("tag deleted")
	w.WriteHeader(http.StatusNoContent)
}
