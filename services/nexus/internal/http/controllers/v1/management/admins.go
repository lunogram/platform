package v1

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/pkg/claim"
	"github.com/lunogram/platform/pkg/http/json"
	"github.com/lunogram/platform/pkg/http/problem"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"go.uber.org/zap"
)

func NewAdminsController(logger *zap.Logger, db *sqlx.DB) *AdminsController {
	return &AdminsController{
		logger: logger,
		db:     db,
		store:  store.NewState(db),
	}
}

type AdminsController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *store.State
}

func (srv *AdminsController) GetProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	logger := srv.logger.With(zap.String("subject", session.Subject))
	logger.Info("getting profile")

	admin, err := srv.store.GetAdminBySubject(ctx, session)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("admin not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("admin not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("profile retrieved")
	json.Write(w, http.StatusOK, admin.OAPI())
}

func (srv *AdminsController) Whoami(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	logger := srv.logger.With(zap.String("subject", session.Subject))
	logger.Info("getting current admin")

	admin, err := srv.store.GetAdminBySubject(ctx, session)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("admin not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("admin not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("current admin retrieved")
	json.Write(w, http.StatusOK, admin.OAPI())
}

func (srv *AdminsController) ListAdmins(w http.ResponseWriter, r *http.Request, params oapi.ListAdminsParams) {
	ctx := r.Context()
	session, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	admin, err := srv.store.GetAdminBySubject(ctx, session)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Error("admin not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("admin not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("organization_id", admin.OrganizationID.String()),
		zap.String("admin_id", admin.ID.String()),
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

	logger.Info("listing admins", zap.Int("limit", pagination.Limit), zap.Int("offset", pagination.Offset))

	admins, total, err := srv.store.ListAdmins(ctx, admin.OrganizationID, pagination, search)
	if err != nil {
		logger.Error("failed to list admins", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	results := make([]oapi.Admin, len(admins))
	for i, a := range admins {
		results[i] = a.OAPI()
	}

	logger.Info("admins listed", zap.Int("total", total), zap.Int("count", len(results)))

	response := oapi.AdminList{
		Results: results,
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
	}

	json.Write(w, http.StatusOK, response)
}

func (srv *AdminsController) CreateAdmin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	currentAdmin, err := srv.store.GetAdminBySubject(ctx, session)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Error("admin not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("admin not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	var body oapi.CreateAdmin
	if err := json.Decode(r.Body, &body); err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(
		zap.String("organization_id", currentAdmin.OrganizationID.String()),
		zap.String("email", string(body.Email)),
		zap.String("role", string(body.Role)),
	)

	if !srv.hasPermission(currentAdmin.Role, string(body.Role)) {
		logger.Error("insufficient permissions")
		oapi.WriteProblem(w, problem.ErrForbidden(problem.Describe("insufficient permissions to assign this role")))
		return
	}

	logger.Info("creating or updating admin")

	existingAdmin, err := srv.store.GetAdminByEmail(ctx, string(body.Email), currentAdmin.OrganizationID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		logger.Error("failed to check existing admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if existingAdmin != nil {
		logger = logger.With(zap.String("admin_id", existingAdmin.ID.String()))
		logger.Info("updating existing admin")

		email := string(body.Email)
		role := string(body.Role)

		update := store.AdminUpdate{
			Email:     &email,
			FirstName: body.FirstName,
			LastName:  body.LastName,
			Role:      &role,
		}

		err = srv.store.UpdateAdmin(ctx, existingAdmin.ID, update)
		if err != nil {
			logger.Error("failed to update admin", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}

		updatedAdmin, err := srv.store.GetAdmin(ctx, existingAdmin.ID)
		if err != nil {
			logger.Error("failed to get updated admin", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}

		logger.Info("admin updated")
		json.Write(w, http.StatusCreated, updatedAdmin.OAPI())
		return
	}

	newAdmin := store.Admin{
		OrganizationID: currentAdmin.OrganizationID,
		Email:          string(body.Email),
		FirstName:      body.FirstName,
		LastName:       body.LastName,
		Role:           string(body.Role),
	}

	adminID, err := srv.store.CreateAdmin(ctx, newAdmin)
	if err != nil {
		logger.Error("failed to create admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	createdAdmin, err := srv.store.GetAdmin(ctx, adminID)
	if err != nil {
		logger.Error("failed to get created admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("admin created", zap.String("admin_id", adminID.String()))
	json.Write(w, http.StatusCreated, createdAdmin.OAPI())
}

func (srv *AdminsController) GetAdmin(w http.ResponseWriter, r *http.Request, adminID uuid.UUID) {
	ctx := r.Context()
	session, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	currentAdmin, err := srv.store.GetAdminBySubject(ctx, session)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Error("admin not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("admin not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("organization_id", currentAdmin.OrganizationID.String()),
		zap.String("admin_id", adminID.String()),
	)

	logger.Info("getting admin")

	admin, err := srv.store.GetAdmin(ctx, adminID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("admin not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("admin not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if admin.OrganizationID != currentAdmin.OrganizationID {
		logger.Error("admin not in organization")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("admin not found")))
		return
	}

	logger.Info("admin retrieved")
	json.Write(w, http.StatusOK, admin.OAPI())
}

func (srv *AdminsController) UpdateAdmin(w http.ResponseWriter, r *http.Request, adminID uuid.UUID) {
	ctx := r.Context()
	session, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	currentAdmin, err := srv.store.GetAdminBySubject(ctx, session)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Error("admin not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("admin not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	admin, err := srv.store.GetAdmin(ctx, adminID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Error("admin not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("admin not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if admin.OrganizationID != currentAdmin.OrganizationID {
		srv.logger.Error("admin not in organization")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("admin not found")))
		return
	}

	var body oapi.UpdateAdmin
	if err := json.Decode(r.Body, &body); err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(
		zap.String("organization_id", currentAdmin.OrganizationID.String()),
		zap.String("admin_id", adminID.String()),
	)

	if body.Role != nil && !srv.hasPermission(currentAdmin.Role, string(*body.Role)) {
		logger.Error("insufficient permissions")
		oapi.WriteProblem(w, problem.ErrForbidden(problem.Describe("insufficient permissions to assign this role")))
		return
	}

	logger.Info("updating admin")

	var email *string
	var role *string

	if body.Email != nil {
		emailStr := string(*body.Email)
		email = &emailStr
	}

	if body.Role != nil {
		roleStr := string(*body.Role)
		role = &roleStr
	}

	update := store.AdminUpdate{
		Email:     email,
		FirstName: body.FirstName,
		LastName:  body.LastName,
		Role:      role,
	}

	err = srv.store.UpdateAdmin(ctx, adminID, update)
	if err != nil {
		logger.Error("failed to update admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	updatedAdmin, err := srv.store.GetAdmin(ctx, adminID)
	if err != nil {
		logger.Error("failed to get updated admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("admin updated")
	json.Write(w, http.StatusOK, updatedAdmin.OAPI())
}

func (srv *AdminsController) DeleteAdmin(w http.ResponseWriter, r *http.Request, adminID uuid.UUID) {
	ctx := r.Context()
	session, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	currentAdmin, err := srv.store.GetAdminBySubject(ctx, session)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Error("admin not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("admin not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	admin, err := srv.store.GetAdmin(ctx, adminID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Error("admin not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("admin not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if admin.OrganizationID != currentAdmin.OrganizationID {
		srv.logger.Error("admin not in organization")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("admin not found")))
		return
	}

	logger := srv.logger.With(
		zap.String("organization_id", currentAdmin.OrganizationID.String()),
		zap.String("admin_id", adminID.String()),
	)

	if !srv.hasPermission(currentAdmin.Role, admin.Role) {
		logger.Error("insufficient permissions")
		oapi.WriteProblem(w, problem.ErrForbidden(problem.Describe("insufficient permissions to delete this admin")))
		return
	}

	logger.Info("deleting admin")

	err = srv.store.DeleteAdmin(ctx, adminID)
	if err != nil {
		logger.Error("failed to delete admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("admin deleted")
	w.WriteHeader(http.StatusNoContent)
}

func (srv *AdminsController) hasPermission(currentRole, targetRole string) bool {
	roles := []string{"member", "admin", "owner"}

	currentIndex := -1
	targetIndex := -1

	for i, role := range roles {
		if role == currentRole {
			currentIndex = i
		}
		if role == targetRole {
			targetIndex = i
		}
	}

	return currentIndex >= targetIndex
}

// Project Admin methods

func (srv *AdminsController) ListProjectAdmins(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListProjectAdminsParams) {
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

	logger.Info("project admins listed", zap.Int("total", total), zap.Int("count", len(projectAdmins)))

	response := oapi.ProjectAdminList{
		Results: store.ProjectAdmins(projectAdmins).OAPI(),
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
	}

	json.Write(w, http.StatusOK, response)
}

func (srv *AdminsController) GetProjectAdmin(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, adminID uuid.UUID) {
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

func (srv *AdminsController) UpdateProjectAdmin(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, adminID uuid.UUID) {
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

func (srv *AdminsController) DeleteProjectAdmin(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, adminID uuid.UUID) {
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
