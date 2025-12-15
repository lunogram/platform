package v1

import (
	"bytes"
	"io"
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/pkg/claim/rbac"
	"github.com/lunogram/platform/pkg/http/json"
	"github.com/lunogram/platform/pkg/http/problem"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/public/oapi"
	"github.com/lunogram/platform/services/nexus/internal/rules"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"go.uber.org/zap"
)

func NewClientController(logger *zap.Logger, db *sqlx.DB, platformProxy http.Handler) *ClientController {
	return &ClientController{
		logger:        logger,
		db:            db,
		store:         store.NewStores(db),
		platformProxy: platformProxy,
	}
}

type ClientController struct {
	logger        *zap.Logger
	db            *sqlx.DB
	store         *store.Stores
	platformProxy http.Handler
}

func (srv *ClientController) PostEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	scope := rbac.FromContext(ctx)
	if scope == nil {
		srv.logger.Error("rbac scope not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	if scope.ProjectID == nil {
		srv.logger.Error("project ID not found in rbac scope")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	projectID := *scope.ProjectID

	logger := srv.logger.With(zap.String("path", r.URL.Path), zap.Stringer("project_id", projectID))
	logger.Info("posting events")

	// TODO: remove after migration has been completed
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error("failed to read request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("failed to read request body")))
		return
	}
	r.Body.Close()

	var events oapi.PostEventsRequest
	err = json.Unmarshal(bodyBytes, &events)
	if err != nil {
		logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	tx, err := srv.db.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	defer tx.Rollback() //nolint:errcheck

	eventsStore := store.NewEventsStore(tx)

	logger.Info("events received", zap.Int("count", len(events)))
	for _, event := range events {
		var data map[string]any
		if event.Data != nil {
			data = *event.Data
		}

		eventID, err := eventsStore.UpsertEvent(ctx, projectID, event.Name)
		if err != nil {
			logger.Error("failed to upsert event", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}

		paths := rules.ParsePaths(data)
		err = eventsStore.UpsertEventSchema(ctx, projectID, eventID, paths)
		if err != nil {
			logger.Error("failed to upsert event schema", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		logger.Error("failed to commit transaction", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	// TODO: remove after migration has been completed
	logger.Info("events processed successfully")
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	srv.platformProxy.ServeHTTP(w, r)
}

func (srv *ClientController) IdentifyUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	scope := rbac.FromContext(ctx)
	if scope == nil {
		srv.logger.Error("rbac scope not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	if scope.ProjectID == nil {
		srv.logger.Error("project ID not found in rbac scope")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	projectID := *scope.ProjectID

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

	user, err := srv.store.UsersStore.IdentifyAndGetUser(ctx, projectID, params, false)
	if err != nil {
		logger.Error("failed to identify user", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	logger.Info("user identified successfully", zap.String("user_id", user.ID.String()))
	json.Write(w, http.StatusOK, user.OAPI())
}
