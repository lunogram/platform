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
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/lunogram/platform/services/nexus/oapi"
	"go.uber.org/zap"
)

func NewAdminsController(logger *zap.Logger, db *sqlx.DB) *AdminsController {
	return &AdminsController{
		logger: logger,
		db:     db,
		store:  store.NewStores(db),
	}
}

type AdminsController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *store.Stores
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

	limit := 20
	offset := 0
	search := ""

	if params.Limit != nil {
		limit = int(*params.Limit)
	}
	if params.Offset != nil {
		offset = int(*params.Offset)
	}
	if params.Search != nil {
		search = *params.Search
	}

	logger.Info("listing admins", zap.Int("limit", limit), zap.Int("offset", offset))

	admins, total, err := srv.store.ListAdmins(ctx, admin.OrganizationID, limit, offset, search)
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
	json.Write(w, http.StatusOK, map[string]interface{}{
		"results": results,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
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
		logger.Info("admin already exists, updating")

		email := string(body.Email)
		role := string(body.Role)

		err = srv.store.UpdateAdmin(ctx, existingAdmin.ID, &email, body.FirstName, body.LastName, &role)
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

	err = srv.store.UpdateAdmin(ctx, adminID, email, body.FirstName, body.LastName, role)
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
