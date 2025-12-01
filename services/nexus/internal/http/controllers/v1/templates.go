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

func NewTemplatesController(logger *zap.Logger, db *sqlx.DB) *TemplatesController {
	return &TemplatesController{
		logger: logger,
		db:     db,
		store:  store.NewStores(db),
	}
}

type TemplatesController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *store.Stores
}

func (srv *TemplatesController) GetTemplate(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, campaignID uuid.UUID, templateID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID), zap.Stringer("template_id", templateID))
	logger.Info("getting template")

	template, err := srv.store.TemplatesStore.GetTemplate(ctx, projectID, templateID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("template not found", zap.Stringer("template_id", templateID))
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
	body := oapi.CreateTemplate{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	campaign, err := srv.store.CampaignsStore.GetCampaign(ctx, projectID, campaignID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Error("campaign not found", zap.Stringer("campaign_id", campaignID))
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
		update := store.TemplateUpdate{
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
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID), zap.Stringer("template_id", templateID))
	logger.Info("deleting template")

	_, err := srv.store.TemplatesStore.GetTemplate(ctx, projectID, templateID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("template not found", zap.Stringer("template_id", templateID))
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
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID), zap.Stringer("template_id", templateID))
	logger.Info("updating template")

	var body oapi.UpdateTemplate
	if err := json.Decode(r.Body, &body); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err := srv.store.TemplatesStore.GetTemplate(ctx, projectID, templateID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("template not found", zap.Stringer("template_id", templateID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("template not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get template", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	updated := store.TemplateUpdate{
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
