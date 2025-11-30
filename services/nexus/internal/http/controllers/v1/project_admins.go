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

func NewProjectAdminsController(logger *zap.Logger, db *sqlx.DB) *ProjectAdminsController {
	return &ProjectAdminsController{
		logger: logger,
		db:     db,
		store:  store.NewStores(db),
	}
}

type ProjectAdminsController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *store.Stores
}

func (srv *ProjectAdminsController) ListProjectAdmins(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListProjectAdminsParams) {
	ctx := r.Context()

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
	)

	pagination := store.Pagination{
		Limit:  20,
		Offset: 0,
	}

	if params.Limit != nil {
		pagination.Limit = params.Limit.ToInt()
	}
	if params.Offset != nil {
		pagination.Offset = params.Offset.ToInt()
	}

	search := params.Search.ToString()

	logger.Info("listing project admins", zap.Int("limit", pagination.Limit), zap.Int("offset", pagination.Offset))

	projectAdmins, total, err := srv.store.ListProjectAdmins(ctx, projectID, pagination, search)
	if err != nil {
		logger.Error("failed to list project admins", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	results := make([]oapi.ProjectAdmin, len(projectAdmins))
	for i, pa := range projectAdmins {
		results[i] = pa.OAPI()
	}

	logger.Info("project admins listed", zap.Int("total", total), zap.Int("count", len(results)))

	response := oapi.ProjectAdminList{
		Results: results,
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
	}

	json.Write(w, http.StatusOK, response)
}

func (srv *ProjectAdminsController) AddProjectAdmin(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()

	var body oapi.AddProjectAdmin
	if err := json.Decode(r.Body, &body); err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("email", string(body.Email)),
		zap.String("role", string(body.Role)),
	)

	logger.Info("adding admin to project")

	project, err := srv.store.GetProject(ctx, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("project not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("project not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get project", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	admin, err := srv.store.GetAdminByEmail(ctx, string(body.Email), *project.OrganizationID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		logger.Error("failed to check existing admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if admin == nil {
		newAdmin := store.Admin{
			OrganizationID: *project.OrganizationID,
			Email:          string(body.Email),
			Role:           "member",
		}

		adminID, err := srv.store.CreateAdmin(ctx, newAdmin)
		if err != nil {
			logger.Error("failed to create admin", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}

		admin, err = srv.store.GetAdmin(ctx, adminID)
		if err != nil {
			logger.Error("failed to get created admin", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}

		logger = logger.With(zap.String("admin_id", admin.ID.String()))
		logger.Info("created new admin")
	} else {
		logger = logger.With(zap.String("admin_id", admin.ID.String()))
	}

	err = srv.store.AddAdminToProject(ctx, projectID, admin.ID, string(body.Role))
	if err != nil {
		logger.Error("failed to add admin to project", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	projectAdmin, err := srv.store.GetProjectAdmin(ctx, projectID, admin.ID)
	if err != nil {
		logger.Error("failed to get project admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("admin added to project")
	json.Write(w, http.StatusCreated, projectAdmin.OAPI())
}

func (srv *ProjectAdminsController) GetProjectAdmin(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, adminID uuid.UUID) {
	ctx := r.Context()

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("admin_id", adminID.String()),
	)

	logger.Info("getting project admin")

	projectAdmin, err := srv.store.GetProjectAdmin(ctx, projectID, adminID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("project admin not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("project admin not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get project admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("project admin retrieved")
	json.Write(w, http.StatusOK, projectAdmin.OAPI())
}

func (srv *ProjectAdminsController) UpdateProjectAdmin(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, adminID uuid.UUID) {
	ctx := r.Context()

	var body oapi.UpdateProjectAdmin
	if err := json.Decode(r.Body, &body); err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("admin_id", adminID.String()),
		zap.String("role", string(body.Role)),
	)

	logger.Info("updating project admin role")

	_, err := srv.store.GetProjectAdmin(ctx, projectID, adminID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("project admin not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("project admin not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get project admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	err = srv.store.UpdateProjectAdminRole(ctx, projectID, adminID, string(body.Role))
	if err != nil {
		logger.Error("failed to update project admin role", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	updatedProjectAdmin, err := srv.store.GetProjectAdmin(ctx, projectID, adminID)
	if err != nil {
		logger.Error("failed to get updated project admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("project admin role updated")
	json.Write(w, http.StatusOK, updatedProjectAdmin.OAPI())
}

func (srv *ProjectAdminsController) DeleteProjectAdmin(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, adminID uuid.UUID) {
	ctx := r.Context()

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("admin_id", adminID.String()),
	)

	logger.Info("deleting project admin")

	_, err := srv.store.GetProjectAdmin(ctx, projectID, adminID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("project admin not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("project admin not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get project admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	err = srv.store.DeleteProjectAdmin(ctx, projectID, adminID)
	if err != nil {
		logger.Error("failed to delete project admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("project admin deleted")
	w.WriteHeader(http.StatusNoContent)
}
