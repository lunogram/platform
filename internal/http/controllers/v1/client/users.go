package v1

import (
	"errors"
	"net/http"

	"github.com/lunogram/platform/internal/http/controllers/v1/client/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

type UsersController struct {
	*ClientController
}

func NewUsersController(client *ClientController) *UsersController {
	return &UsersController{ClientController: client}
}

func (srv *UsersController) DeleteUserClient(w http.ResponseWriter, r *http.Request) {
	projectID, err := srv.engine.AllowedProject(r.Context(), "users", rbac.Delete)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	ctx := r.Context()

	var req oapi.DeleteUserRequest
	err = json.Decode(r.Body, &req)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	if len(req.Identifier) == 0 {
		srv.logger.Error("at least one identifier is required")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("at least one identifier is required")))
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("deleting user")

	userID, err := srv.users.LookupUserID(ctx, projectID, boundUserIdentifiers(ctx, oapi.ToParams(req.Identifier)))
	if errors.Is(err, subjects.ErrUserNotFound) {
		logger.Info("user not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}
	if err != nil {
		logger.Error("failed to lookup user", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	err = srv.users.DeleteUser(ctx, projectID, userID)
	if err != nil {
		logger.Error("failed to delete user", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	logger.Info("user deleted", zap.String("user_id", userID.String()))
	w.WriteHeader(http.StatusNoContent)
}

func (srv *UsersController) UpsertUserClient(w http.ResponseWriter, r *http.Request) {
	projectID, err := srv.engine.AllowedProject(r.Context(), "users", rbac.Create)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	ctx := r.Context()

	logger := srv.logger.With(zap.String("path", r.URL.Path), zap.Stringer("project_id", projectID))
	logger.Info("identifying user")

	var req oapi.IdentifyRequest
	err = json.Decode(r.Body, &req)
	if err != nil {
		logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	if len(req.Identifier) == 0 {
		logger.Error("at least one identifier is required")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("at least one identifier is required")))
		return
	}

	tx, err := srv.db.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	defer tx.Rollback() //nolint:errcheck
	usersStore := subjects.NewUsersStore(tx)

	var data map[string]any
	if req.Data != nil {
		data = *req.Data
	}

	identifiers := boundUserIdentifiers(ctx, oapi.ToParams(req.Identifier))
	params := subjects.UpsertUserParams{
		Identifiers: identifiers,
		Email:       req.Email,
		Phone:       req.Phone,
		Timezone:    req.Timezone,
		Locale:      req.Locale,
		Data:        data,
	}

	user, err := usersStore.IdentifyAndGetUser(ctx, projectID, params, false)
	if err != nil {
		logger.Error("failed to identify user", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	msg := schemas.User{
		ProjectID:   projectID,
		ID:          user.ID,
		Identifiers: identifiers,
		Email:       user.Email,
		Phone:       user.Phone,
		Timezone:    user.Timezone,
		Locale:      user.Locale,
		Data:        data,
		Version:     user.Version,
	}

	err = srv.pubsub.Publish(ctx, schemas.Subject(schemas.UsersProcess(projectID)), msg)
	if err != nil {
		logger.Error("failed to publish user", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	err = tx.Commit()
	if err != nil {
		logger.Error("failed to commit transaction", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	logger.Info("user identified successfully", zap.String("user_id", user.ID.String()))
	json.Write(w, http.StatusOK, user.OAPI())
}
