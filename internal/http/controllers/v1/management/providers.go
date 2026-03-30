package v1

import (
	"errors"
	"fmt"
	"net/http"
	"time"

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

func NewProvidersController(logger *zap.Logger, db *sqlx.DB, registry *internalProviders.Registry, engine *rbac.Engine, baseURL string) *ProvidersController {
	return &ProvidersController{
		logger:   logger,
		db:       db,
		store:    management.NewState(db),
		registry: registry,
		engine:   engine,
		baseURL:  baseURL,
	}
}

type ProvidersController struct {
	logger   *zap.Logger
	db       *sqlx.DB
	store    *management.State
	registry *internalProviders.Registry
	engine   *rbac.Engine
	baseURL  string
}

func (srv *ProvidersController) ListProviders(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListProvidersParams) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("providers", projectID))
	if err != nil {
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
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("providers", projectID))
	if err != nil {
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

		schema, err := json.Marshal(manifest.Spec.Config)
		if err != nil {
			logger.Error("failed to marshal provider schema", zap.String("module", manifest.Metadata.ID), zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}

		channels := make([]oapi.Channel, len(manifest.Spec.Channels))
		for i, ch := range manifest.Spec.Channels {
			channels[i] = oapi.Channel(ch)
		}

		pm := oapi.ProviderMeta{
			Type:        manifest.Metadata.ID,
			Name:        manifest.Metadata.Title,
			Description: &manifest.Metadata.Description,
			Url:         &manifest.Website,
			Channels:    channels,
			Schema:      json.RawMessage(schema),
		}

		if manifest.Metadata.Icon != "" {
			pm.Icon = &manifest.Metadata.Icon
		}

		if manifest.Metadata.Color != "" {
			pm.Color = &manifest.Metadata.Color
		}

		if manifest.Spec.Locked {
			locked := true
			pm.Locked = &locked
		}

		if rl := manifest.Spec.RateLimit; rl != nil {
			pm.RateLimit = &oapi.ProviderRateLimit{
				Limit:    rl.Limit,
				Interval: rl.Interval,
				Override: rl.Override,
			}
			if pm.RateLimit.Interval == "" {
				pm.RateLimit.Interval = "1s"
			}
			maxRL := providers.ProjectMaxRateLimit.Requests
			pm.MaxRateLimit = &maxRL
		}

		meta = append(meta, pm)
	}

	logger.Info("listed provider meta", zap.Int("count", len(meta)))
	json.Write(w, http.StatusOK, meta)
}

func (srv *ProvidersController) CreateProvider(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, providerType string) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("providers", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.CreateProviderJSONRequestBody{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.String("type", providerType),
		zap.String("name", body.Name),
	)
	logger.Info("creating provider")

	module, exists := srv.registry.Get(providerType)
	if !exists {
		logger.Warn("module not found", zap.String("module", providerType))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("provider module not found")))
		return
	}

	// Derive channels from the module manifest.
	manifest := module.Manifest()

	// Reject rate limit overrides when the manifest does not allow them.
	if body.RateLimit != nil && body.RateLimit.Limit > 0 {
		rl := manifest.Spec.RateLimit
		if rl == nil || !rl.Override {
			logger.Warn("rate limit override not allowed for this provider module", zap.String("module", providerType))
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("this provider does not allow rate limit overrides")))
			return
		}
	}

	if body.RateLimit != nil {
		if body.RateLimit.Limit < 0 {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("rate_limit.limit must not be negative")))
			return
		}
		if _, err := time.ParseDuration(body.RateLimit.Interval); err != nil {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("rate_limit.interval must be a valid Go duration (e.g. \"1s\", \"1m\")")))
			return
		}
	}

	channels := make(management.Channels, len(manifest.Spec.Channels))
	for i, ch := range manifest.Spec.Channels {
		channels[i] = string(ch)
	}

	var data json.RawMessage
	if body.Data != nil {
		data = *body.Data
	}

	// Validate the provider configuration before persisting.
	// If the module does not export a validate() function, this is a no-op.
	valid, err := module.Validate(ctx, providers.ValidateRequest{
		Config: json.RawMessage(data),
	})
	if err != nil {
		logger.Error("provider validation failed", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("provider configuration validation failed")))
		return
	}

	if !valid.Valid {
		logger.Warn("provider configuration invalid", zap.Any("errors", valid.Errors))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe(fmt.Sprintf("invalid provider configuration: %s", valid.Message))))
		return
	}

	provider := management.Provider{
		ProjectID:    projectID,
		Module:       providerType,
		Channels:     channels,
		Name:         body.Name,
		Data:         data,
		LinkWrap:     true,
		RateLimit:    0,
		RateInterval: "1s",
	}

	if body.LinkWrap != nil {
		provider.LinkWrap = *body.LinkWrap
	}

	if body.RateLimit != nil {
		provider.RateLimit = body.RateLimit.Limit
		provider.RateInterval = body.RateLimit.Interval
	}

	providerID, err := srv.store.ProvidersStore.CreateProvider(ctx, provider)
	if err != nil {
		logger.Error("failed to create provider", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	init, err := module.Init(ctx, providers.InitRequest{
		Config:     json.RawMessage(data),
		WebhookURL: providers.WebhookURL(srv.baseURL, projectID, providerID),
		ProviderID: providerID.String(),
		ProjectID:  projectID.String(),
	})
	if err != nil {
		logger.Error("provider init failed", zap.Stringer("provider_id", providerID), zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("provider initialization failed")))
		return
	}

	if len(init.ConfigPatch) > 0 {
		data, err := mergeJSON(data, init.ConfigPatch)
		if err != nil {
			logger.Error("failed to merge init config patch", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}

		patch := json.RawMessage(data)
		err = srv.store.ProvidersStore.UpdateProvider(ctx, projectID, providerID, management.ProviderUpdate{Data: &patch})
		if err != nil {
			logger.Error("failed to persist init config patch", zap.Error(err))
		}
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

func (srv *ProvidersController) GetProvider(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, providerType string, providerID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("providers", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
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

func (srv *ProvidersController) UpdateProvider(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, providerType string, providerID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("providers", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.UpdateProviderJSONRequestBody{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("type", providerType), zap.Stringer("provider_id", providerID))
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

	// Reject rate limit overrides when the manifest does not allow them.
	if body.RateLimit != nil && body.RateLimit.Limit > 0 {
		module, exists := srv.registry.Get(providerType)
		if exists {
			rl := module.Manifest().Spec.RateLimit
			if rl == nil || !rl.Override {
				logger.Warn("rate limit override not allowed for this provider module", zap.String("module", providerType))
				oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("this provider does not allow rate limit overrides")))
				return
			}
		}
	}

	if body.RateLimit != nil {
		if body.RateLimit.Limit < 0 {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("rate_limit.limit must not be negative")))
			return
		}
		if _, err := time.ParseDuration(body.RateLimit.Interval); err != nil {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("rate_limit.interval must be a valid Go duration (e.g. \"1s\", \"1m\")")))
			return
		}
	}

	update := management.ProviderUpdate{
		Name:     body.Name,
		Data:     body.Data,
		LinkWrap: body.LinkWrap,
	}

	if body.RateLimit != nil {
		update.RateLimit = &body.RateLimit.Limit
		update.RateInterval = &body.RateLimit.Interval
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
	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("providers", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("provider_id", providerID))
	logger.Info("deleting provider")

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

	// Check if the provider module is locked and cannot be deleted.
	// Also use the looked-up module for the destroy() call below.
	module, has := srv.registry.Get(provider.Module)
	if has && module.Manifest().Spec.Locked {
		logger.Warn("cannot delete locked provider", zap.String("module", provider.Module))
		oapi.WriteProblem(w, problem.ErrForbidden(problem.Describe("this provider is locked and cannot be deleted")))
		return
	}

	// Destroy the provider (deregister webhooks, clean up external resources).
	// If the module does not export a destroy() function, this is a no-op.
	if has {
		_, err = module.Destroy(ctx, providers.DestroyRequest{
			Config:     json.RawMessage(provider.Data),
			ProviderID: providerID.String(),
			ProjectID:  projectID.String(),
		})
		if err != nil {
			logger.Error("provider destroy failed (proceeding with deletion)",
				zap.Stringer("provider_id", providerID),
				zap.Error(err),
			)
		}
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

// mergeJSON performs a JSON merge-patch: it merges patch fields into base,
// with patch values taking precedence. Both inputs must be JSON objects.
func mergeJSON(base, patch json.RawMessage) (json.RawMessage, error) {
	var baseMap map[string]json.RawMessage
	if len(base) > 0 {
		if err := json.Unmarshal(base, &baseMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal base config: %w", err)
		}
	}
	if baseMap == nil {
		baseMap = make(map[string]json.RawMessage)
	}

	var patchMap map[string]json.RawMessage
	if err := json.Unmarshal(patch, &patchMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config patch: %w", err)
	}

	for k, v := range patchMap {
		baseMap[k] = v
	}

	return json.Marshal(baseMap)
}
