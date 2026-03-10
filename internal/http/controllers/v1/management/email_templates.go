package v1

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/webhook"
	"go.uber.org/zap"
)

func NewEmailTemplatesController(logger *zap.Logger, webhookCaller *webhook.Caller, engine *rbac.Engine) *EmailTemplatesController {
	return &EmailTemplatesController{
		logger:  logger,
		webhook: webhookCaller,
		engine:  engine,
	}
}

type EmailTemplatesController struct {
	logger  *zap.Logger
	webhook *webhook.Caller
	engine  *rbac.Engine
}

func (srv *EmailTemplatesController) ListEmailTemplates(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListEmailTemplatesParams) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("templates", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID))

	// If no webhook is configured, return an empty list
	if !srv.webhook.EmailTemplatesEnabled() {
		logger.Debug("no email templates webhook configured, returning empty list")
		json.Write(w, http.StatusOK, oapi.EmailTemplateListResponse{
			Total:   0,
			Limit:   params.Limit.ToInt(),
			Offset:  params.Offset.ToInt(),
			Results: []oapi.EmailTemplate{},
		})
		return
	}

	logger.Info("proxying email templates request")

	body, err := srv.webhook.EmailTemplates(ctx, r)
	if err != nil {
		logger.Error("failed to fetch email templates from webhook", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Proxy the raw JSON response from the webhook
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}
