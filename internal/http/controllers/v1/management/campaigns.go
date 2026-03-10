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

func NewCampaignsController(logger *zap.Logger, managementDB, usersDB *sqlx.DB, engine *rbac.Engine) *CampaignsController {
	return &CampaignsController{
		logger:  logger,
		mgmtDB:  managementDB,
		usersDB: usersDB,
		mgmt:    management.NewState(managementDB),
		engine:  engine,
	}
}

type CampaignsController struct {
	logger  *zap.Logger
	mgmtDB  *sqlx.DB
	usersDB *sqlx.DB
	mgmt    *management.State
	engine  *rbac.Engine
}

func (srv *CampaignsController) CreateCampaign(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("campaigns", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.CreateCampaignJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("channel", string(body.Channel)))
	logger.Info("creating campaign")

	project, err := srv.mgmt.ProjectsStore.GetProject(ctx, projectID)
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

	if body.ProviderId == nil {
		provider, err := srv.mgmt.ProvidersStore.GetDefaultProviderChannel(ctx, project.ID, string(body.Channel))
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			logger.Error("failed to get default provider", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}

		// NOTE: if no default provider is found (ErrNoRows), we proceed with nil ProviderId.
		// This allows campaign creation to continue even if no provider is set for the channel.
		// Downstream, a nil ProviderId means the campaign will not be associated with any provider,
		// and it is up to later validation or business logic to handle this case (e.g., by rejecting
		// campaigns without a provider, or allowing them for manual assignment).
		if err == nil {
			body.ProviderId = &provider.ID
		}
	}

	tx, err := srv.mgmtDB.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("unexpected error while attempting to start a transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	defer tx.Rollback() //nolint:errcheck

	campaigns := management.NewCampaignsStore(tx)
	templates := management.NewTemplatesStore(tx)

	campaignID, err := campaigns.CreateCampaign(ctx, management.Campaign{
		ProjectID:      project.ID,
		Name:           body.Name,
		Channel:        string(body.Channel),
		ProviderID:     body.ProviderId,
		SubscriptionID: body.SubscriptionId,
	})
	if err != nil {
		logger.Error("failed to create campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// TODO: create audit log

	_, err = templates.CreateTemplate(ctx, project.ID, campaignID, string(body.Channel), project.Locale)
	if err != nil {
		logger.Error("failed to create template", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	err = tx.Commit()
	if err != nil {
		logger.Error("unexpected error while attempting to commit transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("campaign created")
	campaign, err := srv.mgmt.GetCampaign(ctx, projectID, campaignID)
	if err != nil {
		logger.Error("failed to fetch created campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	json.Write(w, http.StatusCreated, campaign.OAPI())
}

func (srv *CampaignsController) ListCampaigns(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListCampaignsParams) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("campaigns", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("listing campaigns")

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	result, total, err := srv.mgmt.ListCampaigns(ctx, projectID, pagination, params.Search.ToString())
	if err != nil {
		logger.Error("failed to list campaigns", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("listed campaigns", zap.Int("count", len(result)))
	json.Write(w, http.StatusOK, oapi.CampaignListResponse{
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		Results: result.OAPI(),
	})
}

func (srv *CampaignsController) GetCampaign(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, campaignID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("campaigns", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID))
	logger.Info("getting campaign")

	campaign, err := srv.mgmt.GetCampaign(ctx, projectID, campaignID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("campaign not found", zap.Stringer("campaign_id", campaignID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("campaign not found")))
		return
	}

	if err != nil {
		logger.Error("failed to fetch campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("campaign retrieved")
	json.Write(w, http.StatusOK, campaign.OAPI())
}

func (srv *CampaignsController) UpdateCampaign(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, campaignID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("campaigns", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID))
	logger.Info("updating campaign")
	body := oapi.UpdateCampaignJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.mgmt.GetCampaign(ctx, projectID, campaignID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("campaign not found", zap.Stringer("campaign_id", campaignID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("campaign not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	updated := management.CampaignUpdate{
		Name:       body.Name,
		ProviderID: body.ProviderId,
	}

	if body.Variables != nil {
		vars := make(management.CampaignVariables, len(*body.Variables))
		for i, v := range *body.Variables {
			vars[i] = management.CampaignVariable{
				Name:    v.Name,
				Default: v.Default,
			}
		}
		updated.Variables = &store.JSONB[management.CampaignVariables]{Data: vars}
	}

	err = srv.mgmt.UpdateCampaign(ctx, projectID, campaignID, updated)
	if err != nil {
		logger.Error("failed to update campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	campaign, err := srv.mgmt.GetCampaign(ctx, projectID, campaignID)
	if err != nil {
		logger.Error("failed to fetch updated campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("campaign updated")
	json.Write(w, http.StatusOK, campaign.OAPI())
}

func (srv *CampaignsController) DeleteCampaign(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, campaignID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("campaigns", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID))
	logger.Info("deleting campaign")

	_, err := srv.mgmt.GetCampaign(ctx, projectID, campaignID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("campaign not found", zap.Stringer("campaign_id", campaignID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("campaign not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	err = srv.mgmt.DeleteCampaign(ctx, projectID, campaignID)
	if err != nil {
		logger.Error("failed to delete campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("campaign deleted")
	w.WriteHeader(http.StatusNoContent)
}

func (srv *CampaignsController) DuplicateCampaign(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, campaignID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("campaigns", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID))
	logger.Info("duplicating campaign")

	campaign, err := srv.mgmt.GetCampaign(ctx, projectID, campaignID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("campaign not found", zap.Stringer("campaign_id", campaignID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("campaign not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	tx, err := srv.mgmtDB.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("unexpected error while attempting to start a transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	defer tx.Rollback() //nolint:errcheck

	campaigns := management.NewCampaignsStore(tx)
	templates := management.NewTemplatesStore(tx)

	newCampaignID, err := campaigns.CreateCampaign(ctx, management.Campaign{
		ProjectID:      campaign.ProjectID,
		Name:           "Copy of " + campaign.Name,
		Channel:        campaign.Channel,
		ProviderID:     campaign.ProviderID,
		SubscriptionID: campaign.SubscriptionID,
	})
	if err != nil {
		logger.Error("failed to create duplicated campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Get all templates from the original campaign
	for _, template := range campaign.Templates {
		err = templates.DuplicateTemplate(ctx, projectID, template.ID, newCampaignID)
		if err != nil {
			logger.Error("failed to duplicate template", zap.Error(err), zap.Stringer("template_id", template.ID))
			oapi.WriteProblem(w, err)
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		logger.Error("unexpected error while attempting to commit transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("campaign duplicated", zap.Stringer("new_campaign_id", newCampaignID))
	duplicated, err := srv.mgmt.GetCampaign(ctx, projectID, newCampaignID)
	if err != nil {
		logger.Error("failed to fetch duplicated campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	json.Write(w, http.StatusCreated, duplicated.OAPI())
}

func (srv *CampaignsController) GetCampaignUsers(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, campaignID uuid.UUID, params oapi.GetCampaignUsersParams) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("campaigns", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID))
	logger.Info("getting campaign users")

	_, err := srv.mgmt.GetCampaign(ctx, projectID, campaignID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("campaign not found", zap.Stringer("campaign_id", campaignID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("campaign not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	users, total, err := srv.mgmt.GetCampaignUsers(ctx, srv.usersDB, projectID, campaignID, pagination)
	if err != nil {
		logger.Error("failed to get campaign users", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("campaign users retrieved", zap.Int("count", len(users)))
	json.Write(w, http.StatusOK, map[string]any{
		"data":   users.OAPI(),
		"total":  total,
		"limit":  pagination.Limit,
		"offset": pagination.Offset,
	})
}
