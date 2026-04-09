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
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

func NewPushProvidersController(logger *zap.Logger, db *sqlx.DB, engine *rbac.Engine) *PushProvidersController {
	return &PushProvidersController{
		logger: logger,
		db:     db,
		store:  management.NewState(db),
		engine: engine,
	}
}

type PushProvidersController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *management.State
	engine *rbac.Engine
}

var validPlatforms = map[oapi.ProjectPushProviderPlatform]bool{
	oapi.ProjectPushProviderPlatformIos:     true,
	oapi.ProjectPushProviderPlatformAndroid: true,
	oapi.ProjectPushProviderPlatformWeb:     true,
}

func (srv *PushProvidersController) ListProjectPushProviders(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID))

	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("push_providers", projectID))
	if err != nil {
		logger.Error("failed to authorize list push providers", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("listing project push providers")

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

	providers, err := srv.store.ProjectPushProvidersStore.ListProjectPushProviders(ctx, projectID)
	if err != nil {
		logger.Error("failed to list push providers", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	json.Write(w, http.StatusOK, map[string]any{
		"results": providers.OAPI(),
	})
}

func (srv *PushProvidersController) UpsertProjectPushProvider(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, platform oapi.ProjectPushProviderPlatform) {
	ctx := r.Context()
	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.String("platform", string(platform)),
	)

	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("push_providers", projectID))
	if err != nil {
		logger.Error("failed to authorize upsert push provider", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if !validPlatforms[platform] {
		logger.Info("invalid platform")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("platform must be 'ios', 'android', or 'web'")))
		return
	}

	body := oapi.UpsertProjectPushProviderJSONRequestBody{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger = logger.With(zap.Stringer("provider_id", body.ProviderId))
	logger.Info("upserting project push provider")

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

	pp, err := srv.store.ProjectPushProvidersStore.UpsertProjectPushProvider(ctx, management.ProjectPushProvider{
		ProjectID:  projectID,
		ProviderID: body.ProviderId,
		Platform:   string(platform),
	})
	if err != nil {
		logger.Error("failed to upsert push provider", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("push provider upserted", zap.Stringer("id", pp.ID))
	json.Write(w, http.StatusOK, pp.OAPI())
}

func (srv *PushProvidersController) DeleteProjectPushProvider(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, platform oapi.ProjectPushProviderPlatform) {
	ctx := r.Context()
	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.String("platform", string(platform)),
	)

	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("push_providers", projectID))
	if err != nil {
		logger.Error("failed to authorize delete push provider", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if !validPlatforms[platform] {
		logger.Info("invalid platform")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("platform must be 'ios', 'android', or 'web'")))
		return
	}

	logger.Info("deleting project push provider")

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

	err = srv.store.ProjectPushProvidersStore.DeleteProjectPushProvider(ctx, projectID, string(platform))
	if err != nil {
		logger.Error("failed to delete push provider", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("push provider deleted")
	w.WriteHeader(http.StatusNoContent)
}
