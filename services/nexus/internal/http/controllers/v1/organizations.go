package v1

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/pkg/claim/rbac"
	"github.com/lunogram/platform/pkg/http/json"
	"github.com/lunogram/platform/pkg/http/problem"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/lunogram/platform/services/nexus/oapi"
	"go.uber.org/zap"
)

func NewOrganizationsController(logger *zap.Logger, db *sqlx.DB) *OrganizationsController {
	return &OrganizationsController{
		logger: logger,
		db:     db,
		store:  store.NewStores(db),
	}
}

type OrganizationsController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *store.Stores
}

func (srv *OrganizationsController) GetOrganization(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	admin := rbac.FromContext(ctx)
	if admin == nil {
		srv.logger.Error("admin not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	logger := srv.logger.With(zap.Stringer("organization_id", admin.OrganizationID))
	logger.Info("getting organization")

	organization, err := srv.store.GetOrganization(ctx, admin.OrganizationID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("organization not found", zap.Stringer("organization_id", admin.OrganizationID))
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

	admin := rbac.FromContext(ctx)
	if admin == nil {
		srv.logger.Error("admin not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	logger := srv.logger.With(zap.Stringer("organization_id", admin.OrganizationID))

	// Fetch full admin to check role
	fullAdmin, err := srv.store.GetAdmin(ctx, admin.ID)
	if err != nil {
		logger.Error("failed to get admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Check if admin has owner role
	if fullAdmin.Role != "owner" {
		logger.Error("admin does not have owner role", zap.String("role", fullAdmin.Role))
		oapi.WriteProblem(w, problem.ErrForbidden(problem.Describe("owner role required")))
		return
	}

	body := oapi.UpdateOrganizationJSONRequestBody{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("updating organization")

	update := store.OrganizationUpdate{
		Username:                  &body.Username,
		Domain:                    body.Domain,
		TrackingDeeplinkMirrorURL: body.TrackingDeeplinkMirrorUrl,
	}

	err = srv.store.UpdateOrganization(ctx, admin.OrganizationID, update)
	if err != nil {
		logger.Error("failed to update organization", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	organization, err := srv.store.GetOrganization(ctx, admin.OrganizationID)
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

	admin := rbac.FromContext(ctx)
	if admin == nil {
		srv.logger.Error("admin not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	logger := srv.logger.With(zap.Stringer("organization_id", admin.OrganizationID))

	// Fetch full admin to check role
	fullAdmin, err := srv.store.GetAdmin(ctx, admin.ID)
	if err != nil {
		logger.Error("failed to get admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Check if admin has owner role
	if fullAdmin.Role != "owner" {
		logger.Error("admin does not have owner role", zap.String("role", fullAdmin.Role))
		oapi.WriteProblem(w, problem.ErrForbidden(problem.Describe("owner role required")))
		return
	}

	logger.Info("deleting organization")

	err = srv.store.DeleteOrganization(ctx, admin.OrganizationID)
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

	admin := rbac.FromContext(ctx)
	if admin == nil {
		srv.logger.Error("admin not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	logger := srv.logger.With(zap.Stringer("organization_id", admin.OrganizationID))
	logger.Info("getting organization integrations")

	providers, err := srv.store.GetOrganizationIntegrations(ctx, admin.OrganizationID)
	if err != nil {
		logger.Error("failed to get organization integrations", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	results := make([]oapi.Provider, len(providers))
	for i, p := range providers {
		results[i] = *p.OAPI()
	}

	logger.Info("organization integrations retrieved", zap.Int("count", len(results)))
	json.Write(w, http.StatusOK, results)
}
