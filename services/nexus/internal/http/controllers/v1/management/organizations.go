package v1

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/pkg/claim/rbac"
	"github.com/lunogram/platform/pkg/http/json"
	"github.com/lunogram/platform/pkg/http/problem"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"go.uber.org/zap"
)

func NewOrganizationsController(logger *zap.Logger, db *sqlx.DB) *OrganizationsController {
	return &OrganizationsController{
		logger: logger,
		db:     db,
		store:  store.NewState(db),
	}
}

type OrganizationsController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *store.State
}

func (srv *OrganizationsController) GetOrganization(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	scope := rbac.FromContext(ctx)
	if scope == nil {
		srv.logger.Error("admin not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	logger := srv.logger.With(zap.Stringer("organization_id", scope.OrganizationID))
	logger.Info("getting organization")

	organization, err := srv.store.GetOrganization(ctx, scope.OrganizationID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("organization not found", zap.Stringer("organization_id", scope.OrganizationID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get organization", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("organization retrieved")
	json.Write(w, http.StatusOK, organization.OAPI())
}

func (srv *OrganizationsController) UpdateOrganization(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	scope := rbac.FromContext(ctx)
	if scope == nil {
		srv.logger.Error("admin not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	logger := srv.logger.With(zap.Stringer("organization_id", scope.OrganizationID))

	body := oapi.UpdateOrganizationJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("updating organization")

	update := store.OrganizationUpdate{
		TrackingDeeplinkMirrorURL: body.TrackingDeeplinkMirrorUrl,
	}

	err = srv.store.UpdateOrganization(ctx, scope.OrganizationID, update)
	if err != nil {
		logger.Error("failed to update organization", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	organization, err := srv.store.GetOrganization(ctx, scope.OrganizationID)
	if err != nil {
		logger.Error("failed to fetch updated organization", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("organization updated")
	json.Write(w, http.StatusOK, organization.OAPI())
}

func (srv *OrganizationsController) DeleteOrganization(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	scope := rbac.FromContext(ctx)
	if scope == nil {
		srv.logger.Error("admin not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	logger := srv.logger.With(zap.Stringer("organization_id", scope.OrganizationID))
	logger.Info("deleting organization")

	err := srv.store.DeleteOrganization(ctx, scope.OrganizationID)
	if err != nil {
		logger.Error("failed to delete organization", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("organization deleted")
	w.WriteHeader(http.StatusNoContent)
}

func (srv *OrganizationsController) GetOrganizationIntegrations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	scope := rbac.FromContext(ctx)
	if scope == nil {
		srv.logger.Error("admin not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	logger := srv.logger.With(zap.Stringer("organization_id", scope.OrganizationID))
	logger.Info("getting organization integrations")

	providers, err := srv.store.GetOrganizationIntegrations(ctx, scope.OrganizationID)
	if err != nil {
		logger.Error("failed to get organization integrations", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("organization integrations retrieved", zap.Int("count", len(providers)))
	json.Write(w, http.StatusOK, providers.OAPI())
}
