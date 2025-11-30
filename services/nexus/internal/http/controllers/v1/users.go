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

func NewUsersController(logger *zap.Logger, db *sqlx.DB) *UsersController {
	return &UsersController{
		logger: logger,
		db:     db,
		store:  store.NewStores(db),
	}
}

type UsersController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *store.Stores
}

func (srv *UsersController) ListUsers(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListUsersParams) {
	ctx := r.Context()
	_, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	logger := srv.logger.With(zap.String("project_id", projectID.String()))

	search := params.Search.ToString()
	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	logger.Info("listing users", zap.Int("limit", pagination.Limit), zap.Int("offset", pagination.Offset))

	users, total, err := srv.store.ListUsers(ctx, projectID, pagination, search)
	if err != nil {
		logger.Error("failed to list users", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("users listed", zap.Int("total", total), zap.Int("count", len(users)))

	response := oapi.UserList{
		Results: users.OAPI(),
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
	}

	json.Write(w, http.StatusOK, response)
}

func (srv *UsersController) IdentifyUser(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	_, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	body := oapi.IdentifyUser{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	if body.AnonymousId == nil && body.ExternalId == nil {
		srv.logger.Error("either anonymous_id or external_id required")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("either anonymous_id or external_id required")))
		return
	}

	logger := srv.logger.With(zap.String("project_id", projectID.String()))
	logger.Info("upserting user")

	var emailStr *string
	if body.Email != nil {
		e := string(*body.Email)
		emailStr = &e
	}

	userID, err := srv.store.UpsertUser(
		ctx,
		projectID,
		body.AnonymousId,
		body.ExternalId,
		emailStr,
		body.Phone,
		body.Timezone,
		body.Locale,
		body.Data,
	)
	if err != nil {
		logger.Error("failed to upsert user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	user, err := srv.store.GetUser(ctx, projectID, userID)
	if err != nil {
		logger.Error("failed to get user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("user upserted", zap.String("user_id", userID.String()))
	json.Write(w, http.StatusOK, user.OAPI())
}

func (srv *UsersController) GetUser(w http.ResponseWriter, r *http.Request, projectID, userID uuid.UUID) {
	ctx := r.Context()
	_, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("user_id", userID.String()),
	)

	logger.Info("getting user")

	user, err := srv.store.GetUser(ctx, projectID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("user not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("user retrieved")
	json.Write(w, http.StatusOK, user.OAPI())
}

func (srv *UsersController) UpdateUser(w http.ResponseWriter, r *http.Request, projectID, userID uuid.UUID) {
	ctx := r.Context()
	_, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	_, err := srv.store.GetUser(ctx, projectID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Error("user not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.UpdateUser{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("user_id", userID.String()),
	)

	logger.Info("updating user")

	update := store.UserUpdate{
		Email:    body.Email,
		Phone:    body.Phone,
		Timezone: body.Timezone,
		Locale:   body.Locale,
		Data:     body.Data,
	}

	err = srv.store.UpdateUser(ctx, userID, update)
	if err != nil {
		logger.Error("failed to update user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	updatedUser, err := srv.store.GetUser(ctx, projectID, userID)
	if err != nil {
		logger.Error("failed to get updated user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("user updated")
	json.Write(w, http.StatusOK, updatedUser.OAPI())
}

func (srv *UsersController) DeleteUser(w http.ResponseWriter, r *http.Request, projectID, userID uuid.UUID) {
	ctx := r.Context()
	_, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	_, err := srv.store.GetUser(ctx, projectID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Error("user not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("user_id", userID.String()),
	)

	logger.Info("deleting user")

	err = srv.store.DeleteUser(ctx, projectID, userID)
	if err != nil {
		logger.Error("failed to delete user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("user deleted")
	w.WriteHeader(http.StatusNoContent)
}
