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
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

func NewSenderIdentitiesController(logger *zap.Logger, db *sqlx.DB, engine *rbac.Engine) *SenderIdentitiesController {
	return &SenderIdentitiesController{
		logger: logger,
		db:     db,
		store:  management.NewState(db),
		engine: engine,
	}
}

type SenderIdentitiesController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *management.State
	engine *rbac.Engine
}

func (srv *SenderIdentitiesController) CreateSenderIdentity(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("sender_identities", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.CreateSenderIdentityJSONRequestBody{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	if body.Channel != oapi.CreateSenderIdentityChannelEmail && body.Channel != oapi.CreateSenderIdentityChannelSms {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("channel must be 'email' or 'sms'")))
		return
	}

	// Address must be provided inside traits.
	if body.Traits == nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("traits is required")))
		return
	}
	address, _ := body.Traits["address"].(string)
	if address == "" {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("traits.address is required")))
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("address", address), zap.String("channel", string(body.Channel)), zap.Stringer("provider_id", body.ProviderId))
	logger.Info("creating sender identity")

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

	// Verify provider exists and belongs to this project
	provider, err := srv.store.ProvidersStore.GetProviderByProject(ctx, projectID, body.ProviderId)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("provider not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("provider not found")))
		return
	}
	if err != nil {
		logger.Error("failed to get provider", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Verify provider channel matches identity channel
	if provider.Channel != string(body.Channel) {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("channel does not match provider channel")))
		return
	}

	// Marshal traits to JSON for storage.
	var traits json.RawMessage
	b, err := json.Marshal(body.Traits)
	if err != nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid traits")))
		return
	}
	traits = b

	identityID, err := srv.store.SenderIdentitiesStore.CreateSenderIdentity(ctx, management.SenderIdentity{
		ProjectID:  projectID,
		ProviderID: body.ProviderId,
		Channel:    string(body.Channel),
		Traits:     traits,
	})
	if err != nil {
		logger.Error("failed to create sender identity", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	identity, err := srv.store.SenderIdentitiesStore.GetSenderIdentity(ctx, projectID, identityID)
	if err != nil {
		logger.Error("failed to get created sender identity", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("sender identity created", zap.Stringer("identity_id", identityID))
	json.Write(w, http.StatusCreated, identity.OAPI())
}

func (srv *SenderIdentitiesController) ListSenderIdentities(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListSenderIdentitiesParams) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("sender_identities", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("listing sender identities")

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

	limit := 20
	if params.Limit != nil {
		limit = params.Limit.ToInt()
	}

	offset := 0
	if params.Offset != nil {
		offset = params.Offset.ToInt()
	}

	var providerID *uuid.UUID
	if params.ProviderId != nil {
		providerID = params.ProviderId
	}

	var channel *string
	if params.Channel != nil {
		ch := string(*params.Channel)
		channel = &ch
	}

	identities, total, err := srv.store.SenderIdentitiesStore.ListSenderIdentities(ctx, projectID, providerID, channel, store.Pagination{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		logger.Error("failed to list sender identities", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	json.Write(w, http.StatusOK, map[string]any{
		"results": identities.OAPI(),
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

func (srv *SenderIdentitiesController) GetSenderIdentity(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, senderIdentityID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("sender_identities", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("sender_identity_id", senderIdentityID))
	logger.Info("getting sender identity")

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

	identity, err := srv.store.SenderIdentitiesStore.GetSenderIdentity(ctx, projectID, senderIdentityID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("sender identity not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("sender identity not found")))
		return
	}
	if err != nil {
		logger.Error("failed to get sender identity", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	json.Write(w, http.StatusOK, identity.OAPI())
}

func (srv *SenderIdentitiesController) DeleteSenderIdentity(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, senderIdentityID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("sender_identities", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("sender_identity_id", senderIdentityID))
	logger.Info("deleting sender identity")

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

	err = srv.store.SenderIdentitiesStore.DeleteSenderIdentity(ctx, projectID, senderIdentityID)
	if err != nil {
		logger.Error("failed to delete sender identity", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("sender identity deleted")
	w.WriteHeader(http.StatusNoContent)
}
