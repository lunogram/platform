package v1

import (
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

func NewApiKeysController(logger *zap.Logger, db *sqlx.DB) *ApiKeysController {
	return &ApiKeysController{
		logger: logger,
		db:     db,
		store:  management.NewState(db),
	}
}

type ApiKeysController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *management.State
}

func (srv *ApiKeysController) CreateApiKey(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
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

	logger.Info("API key created", zap.Stringer("key_id", apiKey.ID))
	json.Write(w, http.StatusCreated, apiKey.OAPI())
}

func (srv *ApiKeysController) ListApiKeys(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListApiKeysParams) {
	ctx := r.Context()
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
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("key_id", keyID))
	logger.Info("updating API key")

	ctx := r.Context()
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
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("key_id", keyID))
	logger.Info("deleting API key")

	err := srv.store.ApiKeysStore.DeleteApiKey(ctx, projectID, keyID)
	if err != nil {
		logger.Error("failed to delete API key", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("API key deleted")
	w.WriteHeader(http.StatusNoContent)
}
