package v1

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/pkg/claim/rbac"
	"github.com/lunogram/platform/pkg/http/json"
	"github.com/lunogram/platform/pkg/http/problem"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/public/oapi"
	"github.com/lunogram/platform/services/nexus/internal/pubsub"
	"github.com/lunogram/platform/services/nexus/internal/pubsub/schemas"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"go.uber.org/zap"
)

func NewClientController(logger *zap.Logger, db *sqlx.DB, pub pubsub.Publisher) *ClientController {
	return &ClientController{
		logger: logger,
		db:     db,
		store:  store.NewState(db),
		pubsub: pub,
	}
}

type ClientController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *store.State
	pubsub pubsub.Publisher
}

func (srv *ClientController) PostEvents(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()

	scope := rbac.FromContext(ctx)
	if scope == nil {
		srv.logger.Error("rbac scope not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

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

		err = srv.pubsub.Publish(ctx, schemas.Subject(schemas.EventsProjectSubject(projectID)), msg)
		if err != nil {
			logger.Error("failed to publish event", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}
	}

	logger.Info("events processed successfully")
	w.WriteHeader(http.StatusAccepted)
}

func (srv *ClientController) IdentifyUser(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()

	scope := rbac.FromContext(ctx)
	if scope == nil {
		srv.logger.Error("rbac scope not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

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
	users := store.NewUsersStore(tx)

	var data map[string]any
	if req.Data != nil {
		data = *req.Data
	}

	params := store.UpsertUserParams{
		AnonymousID: req.AnonymousId,
		ExternalID:  req.ExternalId,
		Email:       req.Email,
		Phone:       req.Phone,
		Timezone:    req.Timezone,
		Locale:      req.Locale,
		Data:        data,
	}

	user, err := users.IdentifyAndGetUser(ctx, projectID, params, false)
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

	err = srv.pubsub.Publish(ctx, schemas.Subject(schemas.UsersProjectSubject(projectID)), msg)
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
