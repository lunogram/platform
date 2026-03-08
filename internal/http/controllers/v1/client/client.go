package v1

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/controllers/v1/client/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

func NewClientController(logger *zap.Logger, db *sqlx.DB, usrs *subjects.State, pub pubsub.Publisher, engine *rbac.Engine) *ClientController {
	return &ClientController{
		logger: logger,
		db:     db,
		users:  usrs,
		pubsub: pub,
		engine: engine,
	}
}

type ClientController struct {
	logger *zap.Logger
	db     *sqlx.DB
	users  *subjects.State
	pubsub pubsub.Publisher
	engine *rbac.Engine
}

func (srv *ClientController) PostEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if actor == nil {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	projectID := actor.ProjectID
	if projectID == uuid.Nil {
		srv.logger.Error("project_id is required")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	if err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("events", projectID)); err != nil {
		oapi.WriteProblem(w, err)
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
		msg := schemas.UserEvent{
			ProjectID:   projectID,
			Name:        event.Name,
			AnonymousId: event.AnonymousId,
			Data:        event.Data,
			ExternalId:  event.ExternalId,
		}

		err = srv.pubsub.Publish(ctx, schemas.Subject(schemas.UserEventsProcess(projectID)), msg)
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
	actor := rbac.FromContext(ctx)
	if actor == nil {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	projectID := actor.ProjectID
	if projectID == uuid.Nil {
		srv.logger.Error("project_id is required")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	if err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("users", projectID)); err != nil {
		oapi.WriteProblem(w, err)
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

func (srv *ClientController) UpsertOrganizationClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if actor == nil {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	projectID := actor.ProjectID
	if projectID == uuid.Nil {
		srv.logger.Error("project_id is required")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	if err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("organizations", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var req oapi.OrganizationRequest
	err := json.Decode(r.Body, &req)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.String("external_id", req.ExternalId),
	)
	logger.Info("upserting organization")

	var data map[string]any
	if req.Data != nil {
		data = *req.Data
	}

	tx, err := srv.db.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	defer tx.Rollback() //nolint:errcheck
	orgsStore := subjects.NewOrganizationsStore(tx)

	params := subjects.UpsertOrganizationParams{
		ExternalID: req.ExternalId,
		Name:       req.Name,
		Data:       data,
	}

	orgID, err := orgsStore.UpsertOrganization(ctx, projectID, params)
	if err != nil {
		logger.Error("failed to upsert organization", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	org, err := orgsStore.GetOrganization(ctx, projectID, orgID)
	if err != nil {
		logger.Error("failed to get organization after upsert", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	// Publish to pubsub for schema extraction
	msg := schemas.Organization{
		ID:         org.ID,
		ProjectID:  projectID,
		ExternalID: org.ExternalID,
		Name:       org.Name,
		Data:       data,
		Version:    org.Version,
	}

	err = srv.pubsub.Publish(ctx, schemas.OrganizationsProcess(projectID), msg)
	if err != nil {
		logger.Error("failed to publish organization", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	err = tx.Commit()
	if err != nil {
		logger.Error("failed to commit transaction", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	logger.Info("organization upserted", zap.String("organization_id", org.ID.String()))
	json.Write(w, http.StatusOK, orgToClientOAPI(org))
}

func (srv *ClientController) AddOrganizationUserClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if actor == nil {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	projectID := actor.ProjectID
	if projectID == uuid.Nil {
		srv.logger.Error("project_id is required")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	if err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("organizations", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var req oapi.OrganizationUserRequest
	err := json.Decode(r.Body, &req)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.String("org_external_id", req.OrganizationExternalId),
		zap.String("user_external_id", req.UserExternalId),
	)
	logger.Info("adding user to organization")

	tx, err := srv.db.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	defer tx.Rollback() //nolint:errcheck
	orgsStore := subjects.NewOrganizationsStore(tx)
	usersStore := subjects.NewUsersStore(tx)

	// Look up organization by external ID
	orgID, err := orgsStore.LookupOrganizationID(ctx, projectID, req.OrganizationExternalId)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("organization not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}
	if err != nil {
		logger.Error("failed to lookup organization", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	// Look up user by external ID
	userID, err := usersStore.LookupUserID(ctx, projectID, &req.UserExternalId, nil)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("user not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}
	if err != nil {
		logger.Error("failed to lookup user", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	var data map[string]any
	if req.Data != nil {
		data = *req.Data
	}

	orgUser, err := orgsStore.UpsertAndGetOrganizationMember(ctx, orgID, userID, data)
	if err != nil {
		logger.Error("failed to add user to organization", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	msg := schemas.OrganizationUser{
		OrganizationID:         orgID,
		OrganizationExternalID: req.OrganizationExternalId,
		UserID:                 userID,
		ProjectID:              projectID,
		Data:                   data,
		Version:                orgUser.Version,
	}

	err = srv.pubsub.Publish(ctx, schemas.OrganizationUsersProcess(projectID), msg)
	if err != nil {
		logger.Error("failed to publish organization user", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	err = tx.Commit()
	if err != nil {
		logger.Error("failed to commit transaction", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	logger.Info("user added to organization")
	w.WriteHeader(http.StatusOK)
}

func (srv *ClientController) RemoveOrganizationUserClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if actor == nil {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	projectID := actor.ProjectID
	if projectID == uuid.Nil {
		srv.logger.Error("project_id is required")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	if err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("organizations", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var req oapi.RemoveOrganizationUserRequest
	err := json.Decode(r.Body, &req)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.String("org_external_id", req.OrganizationExternalId),
		zap.String("user_external_id", req.UserExternalId),
	)
	logger.Info("removing user from organization")

	// Look up organization by external ID
	orgID, err := srv.users.LookupOrganizationID(ctx, projectID, req.OrganizationExternalId)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("organization not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}
	if err != nil {
		logger.Error("failed to lookup organization", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	// Look up user by external ID
	userID, err := srv.users.LookupUserID(ctx, projectID, &req.UserExternalId, nil)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("user not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}
	if err != nil {
		logger.Error("failed to lookup user", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	err = srv.users.RemoveUserFromOrganization(ctx, orgID, userID)
	if err != nil {
		logger.Error("failed to remove user from organization", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	logger.Info("user removed from organization")
	w.WriteHeader(http.StatusNoContent)
}

func (srv *ClientController) PostOrganizationEventsClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if actor == nil {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	projectID := actor.ProjectID
	if projectID == uuid.Nil {
		srv.logger.Error("project_id is required")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	if err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("events", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var events oapi.PostOrganizationEventsRequest
	err := json.Decode(r.Body, &events)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Int("events", len(events)))
	logger.Info("posting organization events")

	for _, event := range events {
		// Look up organization by external ID
		orgID, err := srv.users.LookupOrganizationID(ctx, projectID, event.OrganizationExternalId)
		if errors.Is(err, sql.ErrNoRows) {
			logger.Warn("organization not found, skipping event",
				zap.String("org_external_id", event.OrganizationExternalId),
				zap.String("event_name", event.Name))
			continue
		}
		if err != nil {
			logger.Error("failed to lookup organization", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}

		var data map[string]any
		if event.Data != nil {
			data = *event.Data
		}

		msg := schemas.OrganizationEvent{
			Name:                   event.Name,
			ProjectID:              projectID,
			OrganizationID:         orgID,
			OrganizationExternalID: event.OrganizationExternalId,
			Data:                   data,
		}

		err = srv.pubsub.Publish(ctx, schemas.OrganizationEventsProcess(projectID), msg)
		if err != nil {
			logger.Error("failed to publish organization event", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}
	}

	logger.Info("organization events processed successfully")
	w.WriteHeader(http.StatusAccepted)
}

// orgToClientOAPI converts a subjects.Organization to client oapi.Organization
func orgToClientOAPI(org *subjects.Organization) oapi.Organization {
	var data map[string]any
	if org.Data != nil {
		_ = json.Unmarshal(org.Data, &data)
	}
	if data == nil {
		data = make(map[string]any)
	}

	return oapi.Organization{
		Id:         org.ID,
		ProjectId:  org.ProjectID,
		ExternalId: org.ExternalID,
		Name:       org.Name,
		Data:       data,
		Version:    org.Version,
		CreatedAt:  org.CreatedAt,
		UpdatedAt:  org.UpdatedAt,
	}
}
