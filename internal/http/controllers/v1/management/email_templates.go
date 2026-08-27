package v1

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/gallery"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/rbac"
	"go.uber.org/zap"
)

func NewEmailTemplatesController(logger *zap.Logger, templates *gallery.Client, engine *rbac.Engine) *EmailTemplatesController {
	return &EmailTemplatesController{
		logger:  logger,
		gallery: templates,
		engine:  engine,
	}
}

type EmailTemplatesController struct {
	logger  *zap.Logger
	gallery *gallery.Client
	engine  *rbac.Engine
}

func (srv *EmailTemplatesController) ListEmailTemplates(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListEmailTemplatesParams) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("templates", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID))

	limit := params.Limit.ToInt()
	offset := params.Offset.ToInt()

	if !srv.gallery.Enabled() {
		logger.Debug("no email template gallery configured, returning empty list")
		json.Write(w, http.StatusOK, oapi.EmailTemplateListResponse{
			Limit:   limit,
			Offset:  offset,
			Results: []oapi.EmailTemplate{},
		})
		return
	}

	query := gallery.Query{Limit: &limit, Offset: &offset}
	if params.Search != nil {
		search := string(*params.Search)
		query.Search = &search
	}

	// The gallery response is decoded and re-encoded rather than streamed
	// through: the endpoint is operator-configured and its payload is rendered
	// in the console, so its bytes are not the platform's to relay unread.
	listing, err := srv.gallery.List(ctx, query)
	if err != nil {
		logger.Error("failed to fetch email templates", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	json.Write(w, http.StatusOK, toEmailTemplateList(listing))
}

// toEmailTemplateList maps the gallery client's domain types onto the
// management API response. The gallery does not speak the API's wire format;
// translating is this layer's job.
func toEmailTemplateList(listing *gallery.Listing) oapi.EmailTemplateListResponse {
	results := make([]oapi.EmailTemplate, 0, len(listing.Results))
	for _, template := range listing.Results {
		results = append(results, oapi.EmailTemplate{
			Id:          template.ID,
			Label:       template.Label,
			Description: template.Description,
			Html:        template.HTML,
			Text:        template.Text,
			Thumbnail:   template.Thumbnail,
			Blocks:      template.Blocks,
		})
	}

	return oapi.EmailTemplateListResponse{
		Total:   listing.Total,
		Limit:   listing.Limit,
		Offset:  listing.Offset,
		Results: results,
	}
}
