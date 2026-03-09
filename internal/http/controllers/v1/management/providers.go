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
	"github.com/lunogram/platform/pkg/modules/providers"
	"go.uber.org/zap"

	internalProviders "github.com/lunogram/platform/internal/providers"
)

func NewProvidersController(logger *zap.Logger, db *sqlx.DB, registry *internalProviders.Registry, engine *rbac.Engine) *ProvidersController {
	return &ProvidersController{
		logger:   logger,
		db:       db,
		store:    management.NewState(db),
		registry: registry,
		engine:   engine,
	}
}

type ProvidersController struct {
	logger   *zap.Logger
	db       *sqlx.DB
	store    *management.State
	registry *internalProviders.Registry
	engine   *rbac.Engine
}

func (srv *ProvidersController) ListProviders(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListProvidersParams) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("providers", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("listing providers")

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	result, total, err := srv.store.ProvidersStore.ListProviders(ctx, projectID, pagination)
	if err != nil {
		logger.Error("failed to list providers", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("listed providers", zap.Int("count", len(result)))
	json.Write(w, http.StatusOK, oapi.ProviderListResponse{
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		Results: result.OAPI(),
	})
}

func (srv *ProvidersController) ListProviderMeta(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("providers", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("listing provider meta")

	allProviders := srv.registry.All()
	meta := make([]oapi.ProviderMeta, 0, len(allProviders))

	for _, p := range allProviders {
		manifest := p.Manifest()

		// Skip hidden modules from the UI listing
		if manifest.Metadata.Hidden {
			continue
		}

		for _, channel := range manifest.Spec.Channels {
			schema, err := json.Marshal(manifest.Spec.Config)
			if err != nil {
				logger.Error("failed to marshal provider schema", zap.String("module", manifest.Metadata.ID), zap.String("channel", string(channel)), zap.Error(err))
				oapi.WriteProblem(w, err)
				return
			}

			meta = append(meta, oapi.ProviderMeta{
				Type:        manifest.Metadata.ID,
				Name:        manifest.Metadata.Title,
				Description: &manifest.Metadata.Description,
				Url:         &manifest.Website,
				Group:       string(channel),
				Schema:      json.RawMessage(schema),
			})
		}
	}

	logger.Info("listed provider meta", zap.Int("count", len(meta)))
	json.Write(w, http.StatusOK, meta)
}

func (srv *ProvidersController) CreateProvider(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, group string, providerType string) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("providers", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.CreateProviderJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.String("group", group),
		zap.String("type", providerType),
		zap.String("name", body.Name),
	)
	logger.Info("creating provider")

	channel := providers.Channel(group)
	if !srv.registry.SupportsChannel(providerType, channel) {
		logger.Warn("module does not support channel", zap.String("module", providerType), zap.String("channel", string(channel)))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("module does not support the specified channel")))
		return
	}

	_, exists := srv.registry.Get(providerType)
	if !exists {
		logger.Warn("module not found", zap.String("module", providerType))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("provider module not found")))
		return
	}

	var data json.RawMessage
	if body.Data != nil {
		data = *body.Data
	}

	provider := management.Provider{
		ProjectID: projectID,
		Module:    providerType,
		Channel:   string(channel),
		Name:      body.Name,
		Data:      data,
	}

	if body.IsDefault != nil {
		provider.IsDefault = *body.IsDefault
	}

	providerID, err := srv.store.ProvidersStore.CreateProvider(ctx, provider)
	if err != nil {
		logger.Error("failed to create provider", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	created, err := srv.store.ProvidersStore.GetProviderByProject(ctx, projectID, providerID)
	if err != nil {
		logger.Error("failed to fetch created provider", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("provider created", zap.Stringer("provider_id", providerID))
	json.Write(w, http.StatusCreated, created.OAPI())
}

func (srv *ProvidersController) GetProvider(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, group string, providerType string, providerID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("providers", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.String("group", group),
		zap.String("type", providerType),
		zap.Stringer("provider_id", providerID),
	)
	logger.Info("getting provider")

	provider, err := srv.store.ProvidersStore.GetProviderByProject(ctx, projectID, providerID)
	if errors.Is(err, store.ErrNoRows) {
		logger.Info("provider not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("provider not found")))
		return
	}

	if err != nil {
		logger.Error("failed to fetch provider", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("provider retrieved")
	json.Write(w, http.StatusOK, provider.OAPI())
}

func (srv *ProvidersController) UpdateProvider(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, group string, providerType string, providerID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("providers", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.UpdateProviderJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("group", group), zap.String("type", providerType), zap.Stringer("provider_id", providerID))
	logger.Info("updating provider")

	_, err = srv.store.ProvidersStore.GetProviderByProject(ctx, projectID, providerID)
	if errors.Is(err, store.ErrNoRows) {
		logger.Info("provider not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("provider not found")))
		return
	}

	if err != nil {
		logger.Error("failed to fetch provider", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	update := management.ProviderUpdate{
		Name:      body.Name,
		Data:      body.Data,
		IsDefault: body.IsDefault,
	}

	err = srv.store.ProvidersStore.UpdateProvider(ctx, projectID, providerID, update)
	if err != nil {
		logger.Error("failed to update provider", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	provider, err := srv.store.ProvidersStore.GetProviderByProject(ctx, projectID, providerID)
	if err != nil {
		logger.Error("failed to fetch updated provider", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("provider updated")
	json.Write(w, http.StatusOK, provider.OAPI())
}

func (srv *ProvidersController) DeleteProvider(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, providerID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("providers", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("provider_id", providerID))
	logger.Info("deleting provider")

	_, err := srv.store.ProvidersStore.GetProviderByProject(ctx, projectID, providerID)
	if errors.Is(err, store.ErrNoRows) {
		logger.Info("provider not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("provider not found")))
		return
	}

	if err != nil {
		logger.Error("failed to fetch provider", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	err = srv.store.ProvidersStore.DeleteProvider(ctx, projectID, providerID)
	if err != nil {
		logger.Error("failed to delete provider", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("provider deleted")
	w.WriteHeader(http.StatusNoContent)
}
