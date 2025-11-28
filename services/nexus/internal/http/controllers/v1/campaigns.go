package v1

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/pkg/http/json"
	"github.com/lunogram/platform/pkg/http/problem"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/lunogram/platform/services/nexus/oapi"
	"go.uber.org/zap"
)

func NewCampaignsController(logger *zap.Logger, db *sqlx.DB) *CampaignsController {
	return &CampaignsController{
		logger: logger,
		db:     db,
		store:  store.NewStores(db),
	}
}

type CampaignsController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *store.Stores
}

func (srv *CampaignsController) CreateCampaign(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	body := oapi.CreateCampaignJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("channel", string(body.Channel)))
	logger.Info("creating campaign")

	project, err := srv.store.ProjectsStore.GetProject(ctx, projectID)
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

	if body.ProviderId == nil {
		provider, err := srv.store.ProvidersStore.GetDefaultProviderChannel(ctx, project.ID, string(body.Channel))
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

	tx, err := srv.db.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("unexpected error while attempting to start a transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	defer tx.Rollback() //nolint:errcheck

	campaigns := store.NewCampaignsStore(tx)
	templates := store.NewTemplatesStore(tx)

	campaignID, err := campaigns.CreateCampaign(ctx, store.Campaign{
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
	campaign, err := srv.store.GetCampaign(ctx, projectID, campaignID)
	if err != nil {
		logger.Error("failed to fetch created campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	json.Write(w, http.StatusCreated, campaign.OAPI())
}

func (srv *CampaignsController) ListCampaigns(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListCampaignsParams) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("listing campaigns")

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	result, total, err := srv.store.ListCampaigns(ctx, projectID, pagination)
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
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID))
	logger.Info("getting campaign")

	campaign, err := srv.store.GetCampaign(ctx, projectID, campaignID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("campaign not found", zap.Stringer("campaign_id", campaignID))
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
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID))
	logger.Info("updating campaign")

	ctx := r.Context()
	body := oapi.UpdateCampaignJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.store.GetCampaign(ctx, projectID, campaignID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("campaign not found", zap.Stringer("campaign_id", campaignID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("campaign not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	updated := store.CampaignUpdate{
		Name:       body.Name,
		ProviderID: body.ProviderId,
	}

	err = srv.store.UpdateCampaign(ctx, projectID, campaignID, updated)
	if err != nil {
		logger.Error("failed to update campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	campaign, err := srv.store.GetCampaign(ctx, projectID, campaignID)
	if err != nil {
		logger.Error("failed to fetch updated campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("campaign updated")
	json.Write(w, http.StatusOK, campaign.OAPI())
}
