package v1

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/claim/rbac"
	"github.com/lunogram/platform/internal/http/controllers/v1/client/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

func NewClientController(logger *zap.Logger, db *sqlx.DB, usrs *subjects.State, pub pubsub.Publisher) *ClientController {
	return &ClientController{
		logger: logger,
		db:     db,
		users:  usrs,
		pubsub: pub,
	}
}

type ClientController struct {
	logger *zap.Logger
	db     *sqlx.DB
	users  *subjects.State
	pubsub pubsub.Publisher
}

func (srv *ClientController) PostEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	scope := rbac.FromContext(ctx)
	if scope == nil {
		srv.logger.Error("rbac scope not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	projectID := scope.ProjectID
	if projectID == uuid.Nil {
		srv.logger.Error("project_id is required")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	var events oapi.PostEventsRequest
	err := json.Decode(r.Body, &events)
	if err != nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Int("events", len(events)))
	logger.Info("posting events")

	for _, event := range events {
		msg := schemas.Event{
			ProjectID:   projectID,
			Name:        event.Name,
			AnonymousId: event.AnonymousId,
			Data:        event.Data,
			ExternalId:  event.ExternalId,
		}

		err = srv.pubsub.Publish(ctx, schemas.Subject(schemas.EventsProcess(projectID)), msg)
		if err != nil {
			logger.Error("failed to publish event", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}
	}

	logger.Info("events processed successfully")
	w.WriteHeader(http.StatusAccepted)
}

func (srv *ClientController) IdentifyUserClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	scope := rbac.FromContext(ctx)
	if scope == nil {
		srv.logger.Error("rbac scope not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	projectID := scope.ProjectID
	if projectID == uuid.Nil {
		srv.logger.Error("project_id is required")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	logger := srv.logger.With(zap.String("path", r.URL.Path), zap.Stringer("project_id", projectID))
	logger.Info("identifying user")

	var req oapi.IdentifyRequest
	err := json.Decode(r.Body, &req)
	if err != nil {
		logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	if req.ExternalId == nil && req.AnonymousId == nil {
		logger.Error("either external_id or anonymous_id is required")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("either external_id or anonymous_id is required")))
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

	params := subjects.UpsertUserParams{
		AnonymousID: req.AnonymousId,
		ExternalID:  req.ExternalId,
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
		AnonymousID: user.AnonymousID,
		ExternalID:  user.ExternalID,
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
