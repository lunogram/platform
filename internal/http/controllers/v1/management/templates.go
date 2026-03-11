package v1

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	stdjson "encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

func NewTemplatesController(logger *zap.Logger, db *sqlx.DB, caller pubsub.Caller, engine *rbac.Engine) *TemplatesController {
	return &TemplatesController{
		logger: logger,
		db:     db,
		store:  management.NewState(db),
		caller: caller,
		engine: engine,
	}
}

type TemplatesController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *management.State
	caller pubsub.Caller
	engine *rbac.Engine
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
		var dataBlob map[string]any
		if err := stdjson.Unmarshal(*body.Data, &dataBlob); err == nil {
			if codeMap, ok := dataBlob["code"].(map[string]any); ok {
				if source, ok := codeMap["source"].(string); ok && source != "" {
					compiledJS, compileErr := srv.compileTemplate(ctx, projectID, source)
					if compileErr != nil {
						logger.Error("failed to compile template", zap.Error(compileErr))
						oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to compile email template")))
						return
					}
					codeMap["bundle"] = compiledJS
					hash := sha256.Sum256([]byte(compiledJS))
					codeMap["bundle_hash"] = hex.EncodeToString(hash[:])
					dataBlob["code"] = codeMap
					updatedData, _ := stdjson.Marshal(dataBlob)
					rawData := stdjson.RawMessage(updatedData)
					body.Data = &rawData
				}
			}
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

// compileTemplate sends the React Email JSX source to the Deno renderer
// service via NATS request/reply and returns the compiled JS bundle.
func (srv *TemplatesController) compileTemplate(ctx context.Context, projectID uuid.UUID, source string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	reply, err := srv.caller.Call(ctx, schemas.EmailCompile(projectID), schemas.CompileEmail{
		Source: source,
	})
	if err != nil {
		return "", err
	}

	var resp schemas.CompileEmailResponse
	if err := stdjson.Unmarshal(reply, &resp); err != nil {
		return "", err
	}

	if resp.Error != "" {
		return "", errors.New(resp.Error)
	}

	return resp.CompiledJS, nil
}
