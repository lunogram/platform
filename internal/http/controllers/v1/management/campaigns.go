package v1

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

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
	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("campaigns", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.CreateCampaignJSONRequestBody{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("channel", string(body.Channel)))
	logger.Info("creating campaign")

	project, err := srv.mgmt.ProjectsStore.GetProject(ctx, projectID, nil)
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

	if body.SubscriptionId != nil {
		subscription, err := srv.mgmt.SubscriptionsStore.GetSubscription(ctx, projectID, *body.SubscriptionId)
		if errors.Is(err, sql.ErrNoRows) {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("subscription not found")))
			return
		}

		if err != nil {
			logger.Error("failed to get subscription", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}

		if subscription.Channel != string(body.Channel) {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("subscription channel does not match campaign channel")))
			return
		}
	}

	transactional := false
	if body.Transactional != nil {
		transactional = *body.Transactional
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
		SubscriptionID: body.SubscriptionId,
		Transactional:  transactional,
	})
	if err != nil {
		logger.Error("failed to create campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// TODO: create audit log

	_, err = templates.CreateTemplate(ctx, project.ID, campaignID, string(body.Channel), project.Locale, "", nil)
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
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("campaigns", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("listing campaigns")

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	archivedOnly := params.IncludeDeleted != nil && *params.IncludeDeleted

	result, total, err := srv.mgmt.ListCampaigns(ctx, projectID, pagination, params.Search.ToString(), archivedOnly)
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
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("campaigns", projectID))
	if err != nil {
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
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("campaigns", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID))
	logger.Info("updating campaign")
	body := oapi.UpdateCampaignJSONRequestBody{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

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

	updated := management.CampaignUpdate{
		Name:           body.Name,
		SubscriptionID: body.SubscriptionId,
		Transactional:  body.Transactional,
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

	if body.Variants != nil {
		if !variantsAvailable {
			oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("template variants are not available in the open-source version")))
			return
		}

		variants, err := management.CampaignVariantsFromOAPI(*body.Variants)
		if err != nil {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe(err.Error())))
			return
		}

		// Templates are keyed by variant, so undeclaring a key - by removing it
		// or by renaming it, which reaches the API as the same edit - would
		// strand its templates: invisible in the console and unreachable by any
		// send. Refuse while they exist rather than orphaning them.
		if orphaned := undeclaredTemplateVariants(campaign.Templates, variants); len(orphaned) > 0 {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe(fmt.Sprintf(
				"variant %s still has templates; delete them before removing or renaming the variant",
				strings.Join(orphaned, ", "),
			))))
			return
		}

		updated.Variants = &store.JSONB[management.CampaignVariants]{Data: variants}
	}

	if body.SubscriptionId != nil {
		subscription, err := srv.mgmt.SubscriptionsStore.GetSubscription(ctx, projectID, *body.SubscriptionId)
		if errors.Is(err, sql.ErrNoRows) {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("subscription not found")))
			return
		}

		if err != nil {
			logger.Error("failed to get subscription", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}

		if subscription.ProjectID != projectID {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("subscription does not belong to this project")))
			return
		}

		if subscription.Channel != campaign.Channel {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("subscription channel does not match campaign channel")))
			return
		}
	}

	err = srv.mgmt.UpdateCampaign(ctx, projectID, campaignID, updated)
	if err != nil {
		logger.Error("failed to update campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	campaign, err = srv.mgmt.GetCampaign(ctx, projectID, campaignID)
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
	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("campaigns", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID))
	logger.Info("deleting campaign")

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

	err = srv.mgmt.DeleteCampaign(ctx, projectID, campaignID)
	if err != nil {
		logger.Error("failed to delete campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("campaign deleted")
	w.WriteHeader(http.StatusNoContent)
}

func (srv *CampaignsController) UnarchiveCampaign(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, campaignID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("campaigns", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID))
	logger.Info("unarchiving campaign")

	err = srv.mgmt.UnarchiveCampaign(ctx, projectID, campaignID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("campaign not found", zap.Stringer("campaign_id", campaignID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("campaign not found")))
		return
	}

	if err != nil {
		logger.Error("failed to unarchive campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("campaign unarchived")
	w.WriteHeader(http.StatusNoContent)
}

func (srv *CampaignsController) DuplicateCampaign(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, campaignID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("campaigns", projectID))
	if err != nil {
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

	// The declared variants travel with the copy: the templates below carry
	// their variant key, and a key the campaign no longer declares hides those
	// templates from the console and drops their sends back to house branding.
	newCampaignID, err := campaigns.CreateCampaign(ctx, management.Campaign{
		ProjectID:      campaign.ProjectID,
		Name:           "Copy of " + campaign.Name,
		Channel:        campaign.Channel,
		SubscriptionID: campaign.SubscriptionID,
		Transactional:  campaign.Transactional,
		Variants:       campaign.Variants,
	})
	if err != nil {
		logger.Error("failed to create duplicated campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

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
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("campaigns", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID))
	logger.Info("getting campaign users")

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

// undeclaredTemplateVariants reports the variant keys that templates still
// carry but the supplied declaration no longer names, sorted for a stable
// error message. The default variant is always declared and never appears.
func undeclaredTemplateVariants(templates management.Templates, variants management.CampaignVariants) []string {
	var orphaned []string
	seen := make(map[string]bool)

	for _, template := range templates {
		if template.Variant == "" || seen[template.Variant] || variants.Has(template.Variant) {
			continue
		}
		seen[template.Variant] = true
		orphaned = append(orphaned, template.Variant)
	}

	sort.Strings(orphaned)
	return orphaned
}
