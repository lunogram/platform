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
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

func NewLocalesController(logger *zap.Logger, db *sqlx.DB) *LocalesController {
	return &LocalesController{
		logger: logger,
		db:     db,
		store:  management.NewState(db),
	}
}

type LocalesController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *management.State
}

func (srv *LocalesController) CreateLocale(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	body := oapi.CreateLocaleJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("key", body.Key))
	logger.Info("creating locale")

	_, err = srv.store.ProjectsStore.GetProject(ctx, projectID)
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

	localeID, err := srv.store.LocalesStore.CreateLocale(ctx, management.Locale{
		ProjectID: projectID,
		Key:       body.Key,
		Label:     body.Label,
	})

	if err != nil {
		logger.Error("failed to create locale", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	locale, err := srv.store.LocalesStore.GetLocale(ctx, projectID, localeID.String())
	if err != nil {
		logger.Error("failed to get created locale", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("locale created", zap.Stringer("locale_id", localeID))
	json.Write(w, http.StatusCreated, map[string]any{
		"data": locale.OAPI(),
	})
}

func (srv *LocalesController) ListLocales(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListLocalesParams) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("listing locales")

	_, err := srv.store.ProjectsStore.GetProject(ctx, projectID)
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

	limit := 20
	if params.Limit != nil {
		limit = params.Limit.ToInt()
	}

	offset := 0
	if params.Offset != nil {
		offset = params.Offset.ToInt()
	}

	locales, total, err := srv.store.LocalesStore.ListLocales(ctx, projectID, store.Pagination{
		Limit:  limit,
		Offset: offset,
	})

	if err != nil {
		logger.Error("failed to list locales", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	json.Write(w, http.StatusOK, map[string]any{
		"results": locales.OAPI(),
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func (srv *LocalesController) GetLocale(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, localeID string) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("locale_id", localeID))
	logger.Info("getting locale")

	_, err := srv.store.ProjectsStore.GetProject(ctx, projectID)
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

	locale, err := srv.store.LocalesStore.GetLocale(ctx, projectID, localeID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("locale not found", zap.String("locale_id", localeID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("locale not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get locale", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	json.Write(w, http.StatusOK, locale.OAPI())
}

func (srv *LocalesController) DeleteLocale(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, localeID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("locale_id", localeID))
	logger.Info("deleting locale")

	_, err := srv.store.ProjectsStore.GetProject(ctx, projectID)
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

	err = srv.store.LocalesStore.DeleteLocale(ctx, projectID, localeID)
	if err != nil {
		logger.Error("failed to delete locale", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("locale deleted")
	w.WriteHeader(http.StatusNoContent)
}
