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
	"github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/providers/channels"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

func NewTemplatesController(logger *zap.Logger, db *sqlx.DB, renderer *pubsub.EmailRenderer, registry *providers.Registry, engine *rbac.Engine) *TemplatesController {
	return &TemplatesController{
		logger:   logger,
		db:       db,
		store:    management.NewState(db),
		renderer: renderer,
		registry: registry,
		engine:   engine,
	}
}

type TemplatesController struct {
	logger   *zap.Logger
	db       *sqlx.DB
	store    *management.State
	renderer *pubsub.EmailRenderer
	registry *providers.Registry
	engine   *rbac.Engine
}

func (srv *TemplatesController) GetTemplate(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, campaignID uuid.UUID, templateID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("templates", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID), zap.Stringer("template_id", templateID))
	logger.Info("getting template")

	template, err := srv.store.TemplatesStore.GetTemplate(ctx, projectID, templateID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("template not found", zap.Stringer("template_id", templateID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("template not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get template", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	json.Write(w, http.StatusOK, template.OAPI())
}

func (srv *TemplatesController) CreateTemplate(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, campaignID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("templates", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}
	body := oapi.CreateTemplate{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	campaign, err := srv.store.CampaignsStore.GetCampaign(ctx, projectID, campaignID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("campaign not found", zap.Stringer("campaign_id", campaignID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("campaign not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID), zap.String("type", campaign.Channel))
	logger.Info("creating template")

	templateID, err := srv.store.TemplatesStore.CreateTemplate(ctx, projectID, campaignID, campaign.Channel, body.Locale)
	if err != nil {
		logger.Error("failed to create template", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if body.Data != nil {
		update := management.TemplateUpdate{
			Data: body.Data,
		}

		err = srv.store.TemplatesStore.UpdateTemplate(ctx, projectID, templateID, update)
		if err != nil {
			logger.Error("failed to update template data", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
	}

	logger.Info("template created")
	template, err := srv.store.TemplatesStore.GetTemplate(ctx, projectID, templateID)
	if err != nil {
		logger.Error("failed to fetch created template", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	json.Write(w, http.StatusCreated, template.OAPI())
}

func (srv *TemplatesController) DeleteTemplate(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, campaignID uuid.UUID, templateID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("templates", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID), zap.Stringer("template_id", templateID))
	logger.Info("deleting template")

	_, err := srv.store.TemplatesStore.GetTemplate(ctx, projectID, templateID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("template not found", zap.Stringer("template_id", templateID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("template not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get template", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	err = srv.store.TemplatesStore.DeleteTemplate(ctx, projectID, templateID)
	if err != nil {
		logger.Error("failed to delete template", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("template deleted")
	w.WriteHeader(http.StatusNoContent)
}

func (srv *TemplatesController) UpdateTemplate(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, campaignID uuid.UUID, templateID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("templates", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID), zap.Stringer("template_id", templateID))
	logger.Info("updating template")

	var body oapi.UpdateTemplate
	if err := json.Decode(r.Body, &body); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err := srv.store.TemplatesStore.GetTemplate(ctx, projectID, templateID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("template not found", zap.Stringer("template_id", templateID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("template not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get template", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// If the template data contains React Email source code, compile it via
	// the Deno renderer service and store the compiled JS alongside the source.
	if body.Data != nil {
		var data channels.EmailTemplateData
		err := json.Unmarshal(*body.Data, &data)
		if err != nil {
			logger.Error("failed to unmarshal template data", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to unmarshal template data")))
			return
		}

		if data.Code.Source != "" {
			data.Code.Bundle, data.Code.BundleHash, err = srv.renderer.Compile(ctx, projectID, data.Code.Source)
			if err != nil {
				logger.Error("failed to compile template", zap.Error(err))
				oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to compile email template")))
				return
			}

			updatedData, _ := json.Marshal(data)
			rawData := json.RawMessage(updatedData)

			body.Data = &rawData
		}
	}

	updated := management.TemplateUpdate{
		Data: body.Data,
	}

	err = srv.store.TemplatesStore.UpdateTemplate(ctx, projectID, templateID, updated)
	if err != nil {
		logger.Error("failed to update template", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	template, err := srv.store.TemplatesStore.GetTemplate(ctx, projectID, templateID)
	if err != nil {
		logger.Error("failed to fetch updated template", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("template updated")
	json.Write(w, http.StatusOK, template.OAPI())
}

func (srv *TemplatesController) SendTest(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, campaignID uuid.UUID, templateID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("templates", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID), zap.Stringer("template_id", templateID))
	logger.Info("sending test")

	var body oapi.SendTest
	if err := json.Decode(r.Body, &body); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	// Get the template.
	template, err := srv.store.TemplatesStore.GetTemplate(ctx, projectID, templateID)
	if errors.Is(err, sql.ErrNoRows) {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("template not found")))
		return
	}
	if err != nil {
		logger.Error("failed to get template", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Get the campaign to find the provider.
	campaign, err := srv.store.CampaignsStore.GetCampaign(ctx, projectID, campaignID)
	if errors.Is(err, sql.ErrNoRows) {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("campaign not found")))
		return
	}
	if err != nil {
		logger.Error("failed to get campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if campaign.ProviderID == nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("campaign has no provider configured")))
		return
	}

	provider, err := srv.store.ProvidersStore.GetProvider(ctx, *campaign.ProviderID)
	if err != nil {
		logger.Error("failed to get provider", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	module, ok := srv.registry.Get(provider.Module)
	if !ok {
		logger.Error("provider module not found", zap.String("module", provider.Module))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("provider module not found")))
		return
	}

	props := make(map[string]any)
	if body.Props != nil {
		props = *body.Props
	}

	templateData, err := channels.ComposeEmailTemplateData(ctx, srv.renderer, projectID, template.Data, props)
	if err != nil {
		logger.Error("failed to compose template data", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to compose email template")))
		return
	}

	var config map[string]any
	err = json.Unmarshal(provider.Data, &config)
	if err != nil {
		logger.Error("failed to parse provider config", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to parse provider config")))
		return
	}

	request, err := channels.ComposePayload(config, templateData, string(body.To))
	if err != nil {
		logger.Error("failed to compose email payload", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe(err.Error())))
		return
	}

	_, err = module.Send(ctx, request)
	if err != nil {
		logger.Error("failed to send test", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to send test: "+err.Error())))
		return
	}

	logger.Info("test sent", zap.String("to", string(body.To)))
	w.WriteHeader(http.StatusNoContent)
}
