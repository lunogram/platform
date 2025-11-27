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

func (srv *TemplatesController) GetTemplate(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, templateID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("template_id", templateID))
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
