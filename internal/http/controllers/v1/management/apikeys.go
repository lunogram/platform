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
	"github.com/lunogram/platform/internal/rbac/access"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

func NewApiKeysController(logger *zap.Logger, db *sqlx.DB, engine *rbac.Engine) *ApiKeysController {
	return &ApiKeysController{
		logger: logger,
		db:     db,
		store:  management.NewState(db),
		engine: engine,
	}
}

type ApiKeysController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *management.State
	engine *rbac.Engine
}

func (srv *ApiKeysController) CreateApiKey(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if err := srv.engine.Allowed(ctx, rbac.Create, rbac.OrganizationScope(actor.OrganizationID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.CreateApiKeyJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("name", body.Name))
	logger.Info("creating API key")

	// Set default role if not provided
	role := "support"
	if body.Role != nil {
		role = string(*body.Role)
	}

	apiKey, err := srv.store.ApiKeysStore.CreateApiKey(ctx, projectID, body.Name, string(body.Scope), role, body.Description)
	if err != nil {
		logger.Error("failed to create API key", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Grant the API key its project role in the RBAC engine so that
	// project-scoped permission checks resolve correctly.
	if err := access.ProvisionApiKey(ctx, srv.engine, apiKey.ID, projectID, role); err != nil {
		logger.Error("failed to provision RBAC for API key", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("API key created", zap.Stringer("key_id", apiKey.ID))
	json.Write(w, http.StatusCreated, apiKey.OAPI())
}

func (srv *ApiKeysController) ListApiKeys(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListApiKeysParams) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if err := srv.engine.Allowed(ctx, rbac.Read, rbac.OrganizationScope(actor.OrganizationID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("listing API keys")

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	result, total, err := srv.store.ApiKeysStore.ListApiKeys(ctx, projectID, pagination)
	if err != nil {
		logger.Error("failed to list API keys", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("listed API keys", zap.Int("count", len(result)))
	json.Write(w, http.StatusOK, oapi.ApiKeyListResponse{
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		Results: result.OAPI(),
	})
}

func (srv *ApiKeysController) GetApiKey(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, keyID uuid.UUID) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if err := srv.engine.Allowed(ctx, rbac.Read, rbac.OrganizationScope(actor.OrganizationID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("key_id", keyID))
	logger.Info("getting API key")

	apiKey, err := srv.store.ApiKeysStore.GetApiKey(ctx, projectID, keyID)
	if errors.Is(err, store.ErrNoRows) {
		logger.Info("API key not found", zap.Stringer("key_id", keyID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("API key not found")))
		return
	}

	if err != nil {
		logger.Error("failed to fetch API key", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("API key retrieved")
	json.Write(w, http.StatusOK, apiKey.OAPI())
}

func (srv *ApiKeysController) UpdateApiKey(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, keyID uuid.UUID) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if err := srv.engine.Allowed(ctx, rbac.Update, rbac.OrganizationScope(actor.OrganizationID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("key_id", keyID))
	logger.Info("updating API key")

	body := oapi.UpdateApiKeyJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	// Convert role to string pointer if provided
	var role *string
	if body.Role != nil {
		roleStr := string(*body.Role)
		role = &roleStr
	}

	// If the role is changing, fetch the current key to get the old role
	// so we can update the RBAC tuple.
	if role != nil {
		existing, err := srv.store.ApiKeysStore.GetApiKey(ctx, projectID, keyID)
		if errors.Is(err, store.ErrNoRows) {
			oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("API key not found")))
			return
		}
		if err != nil {
			logger.Error("failed to fetch API key for role update", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}

		if err := access.UpdateApiKeyRole(ctx, srv.engine, keyID, projectID, existing.Role, *role); err != nil {
			logger.Error("failed to update RBAC role for API key", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
	}

	err = srv.store.ApiKeysStore.UpdateApiKey(ctx, projectID, keyID, body.Name, role, body.Description)
	if err != nil {
		logger.Error("failed to update API key", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	apiKey, err := srv.store.ApiKeysStore.GetApiKey(ctx, projectID, keyID)
	if errors.Is(err, store.ErrNoRows) {
		logger.Info("API key not found", zap.Stringer("key_id", keyID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("API key not found")))
		return
	}

	if err != nil {
		logger.Error("failed to fetch updated API key", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("API key updated")
	json.Write(w, http.StatusOK, apiKey.OAPI())
}

func (srv *ApiKeysController) DeleteApiKey(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, keyID uuid.UUID) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if err := srv.engine.Allowed(ctx, rbac.Delete, rbac.OrganizationScope(actor.OrganizationID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("key_id", keyID))
	logger.Info("deleting API key")

	// Fetch the key before deletion so we know which role tuple to remove.
	apiKey, err := srv.store.ApiKeysStore.GetApiKey(ctx, projectID, keyID)
	if errors.Is(err, store.ErrNoRows) {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("API key not found")))
		return
	}
	if err != nil {
		logger.Error("failed to fetch API key for deletion", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if err := srv.store.ApiKeysStore.DeleteApiKey(ctx, projectID, keyID); err != nil {
		logger.Error("failed to delete API key", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Remove the RBAC role tuple for this key.
	if err := access.DeprovisionApiKey(ctx, srv.engine, keyID, projectID, apiKey.Role); err != nil {
		logger.Error("failed to deprovision RBAC for API key", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("API key deleted")
	w.WriteHeader(http.StatusNoContent)
}
