package v1

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

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

func (srv *ClientController) PostUserEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if actor == nil {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	projectID := actor.ProjectID
	if projectID == uuid.Nil {
		srv.logger.Warn("project_id is required")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("events", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var events oapi.PostEventsRequest
	err = json.Decode(r.Body, &events)
	if err != nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Int("events", len(events)))
	logger.Info("posting events")

	for _, event := range events {
		// match and identifier are mutually exclusive
		if event.Match != nil && event.Identifier != nil {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("match and identifier are mutually exclusive")))
			return
		}
		if event.Match == nil && event.Identifier == nil {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("one of match or identifier is required")))
			return
		}

		switch {
		case event.Match != nil:
			msg := schemas.MatchUserEvent{
				ProjectID: projectID,
				Name:      event.Name,
				Match:     *event.Match,
				Data:      event.Data,
			}
			err = srv.pubsub.Publish(ctx, schemas.UserEventsMatch(projectID), msg)

		default:
			msg := schemas.UserEvent{
				ProjectID: projectID,
				Name:      event.Name,
				Data:      event.Data,
			}
			if event.Identifier != nil {
				msg.Identifiers = oapi.ToParams(*event.Identifier)
			}
			err = srv.pubsub.Publish(ctx, schemas.Subject(schemas.UserEventsProcess(projectID)), msg)
		}

		if err != nil {
			logger.Error("failed to publish event", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}
	}

	logger.Info("events processed successfully")
	w.WriteHeader(http.StatusAccepted)
}

func (srv *ClientController) DeleteUserClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if actor == nil {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	projectID := actor.ProjectID
	if projectID == uuid.Nil {
		srv.logger.Warn("project_id is required")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("users", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

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

	userID, err := srv.users.LookupUserID(ctx, projectID, oapi.ToParams(req.Identifier))
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

func (srv *ClientController) UpsertUserClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if actor == nil {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	projectID := actor.ProjectID
	if projectID == uuid.Nil {
		srv.logger.Warn("project_id is required")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("users", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

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

	identifiers := oapi.ToParams(req.Identifier)
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

func (srv *ClientController) UpsertOrganizationClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if actor == nil {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	projectID := actor.ProjectID
	if projectID == uuid.Nil {
		srv.logger.Warn("project_id is required")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("organizations", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var req oapi.OrganizationRequest
	err = json.Decode(r.Body, &req)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.Int("identifiers", len(req.Identifier)),
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

	orgIdentifiers := oapi.ToParams(req.Identifier)
	params := subjects.UpsertOrganizationParams{
		Identifiers: orgIdentifiers,
		Name:        req.Name,
		Data:        data,
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
		ID:          org.ID,
		ProjectID:   projectID,
		Identifiers: orgIdentifiers,
		Name:        org.Name,
		Data:        data,
		Version:     org.Version,
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

func (srv *ClientController) DeleteOrganizationClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if actor == nil {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	projectID := actor.ProjectID
	if projectID == uuid.Nil {
		srv.logger.Warn("project_id is required")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("organizations", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var req oapi.DeleteOrganizationRequest
	err = json.Decode(r.Body, &req)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.Int("identifiers", len(req.Identifier)),
	)
	logger.Info("deleting organization")

	orgID, err := srv.users.LookupOrganizationID(ctx, projectID, oapi.ToParams(req.Identifier))
	if errors.Is(err, subjects.ErrOrgNotFound) {
		logger.Info("organization not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}
	if err != nil {
		logger.Error("failed to lookup organization", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	err = srv.users.DeleteOrganization(ctx, projectID, orgID)
	if err != nil {
		logger.Error("failed to delete organization", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	logger.Info("organization deleted", zap.String("organization_id", orgID.String()))
	w.WriteHeader(http.StatusNoContent)
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
		srv.logger.Warn("project_id is required")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("organizations", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var req oapi.OrganizationUserRequest
	err = json.Decode(r.Body, &req)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.Int("org_identifiers", len(req.Organization.Identifier)),
		zap.Int("user_identifiers", len(req.User.Identifier)),
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

	// Look up organization by identifiers
	orgID, err := orgsStore.LookupOrganizationID(ctx, projectID, oapi.ToParams(req.Organization.Identifier))
	if errors.Is(err, subjects.ErrOrgNotFound) {
		logger.Info("organization not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}
	if err != nil {
		logger.Error("failed to lookup organization", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	// Look up user by identifiers
	userID, err := usersStore.LookupUserID(ctx, projectID, oapi.ToParams(req.User.Identifier))
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
		OrganizationID:          orgID,
		OrganizationIdentifiers: oapi.ToParams(req.Organization.Identifier),
		UserID:                  userID,
		ProjectID:               projectID,
		Data:                    data,
		Version:                 orgUser.Version,
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
		srv.logger.Warn("project_id is required")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("organizations", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var req oapi.RemoveOrganizationUserRequest
	err = json.Decode(r.Body, &req)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(
		zap.Stringer("project_id", projectID),
		zap.Int("org_identifiers", len(req.Organization.Identifier)),
		zap.Int("user_identifiers", len(req.User.Identifier)),
	)
	logger.Info("removing user from organization")

	// Look up organization by identifiers
	orgID, err := srv.users.LookupOrganizationID(ctx, projectID, oapi.ToParams(req.Organization.Identifier))
	if errors.Is(err, subjects.ErrOrgNotFound) {
		logger.Info("organization not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}
	if err != nil {
		logger.Error("failed to lookup organization", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	// Look up user by identifiers
	userID, err := srv.users.LookupUserID(ctx, projectID, oapi.ToParams(req.User.Identifier))
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
		srv.logger.Warn("project_id is required")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("events", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var events oapi.PostOrganizationEventsRequest
	err = json.Decode(r.Body, &events)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Int("events", len(events)))
	logger.Info("posting organization events")

	for _, event := range events {
		// match and identifier are mutually exclusive
		if event.Match != nil && event.Identifier != nil {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("match and identifier are mutually exclusive")))
			return
		}
		if event.Match == nil && event.Identifier == nil {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("one of match or identifier is required")))
			return
		}

		var data map[string]any
		if event.Data != nil {
			data = *event.Data
		}

		switch {
		case event.Match != nil:
			msg := schemas.MatchOrganizationEvent{
				ProjectID: projectID,
				Name:      event.Name,
				Match:     *event.Match,
				Data:      data,
			}
			err = srv.pubsub.Publish(ctx, schemas.OrganizationEventsMatch(projectID), msg)

		case event.Identifier != nil:
			orgIdentifiers := oapi.ToParams(*event.Identifier)
			var orgID uuid.UUID
			orgID, err = srv.users.LookupOrganizationID(ctx, projectID, orgIdentifiers)
			if errors.Is(err, subjects.ErrOrgNotFound) {
				logger.Warn("organization not found, skipping event",
					zap.Int("org_identifiers", len(*event.Identifier)),
					zap.String("event_name", event.Name))
				continue
			}
			if err != nil {
				logger.Error("failed to lookup organization", zap.Error(err))
				oapi.WriteProblem(w, problem.ErrInternal())
				return
			}

			msg := schemas.OrganizationEvent{
				Name:                    event.Name,
				ProjectID:               projectID,
				OrganizationID:          orgID,
				OrganizationIdentifiers: orgIdentifiers,
				Data:                    data,
			}

			err = srv.pubsub.Publish(ctx, schemas.OrganizationEventsProcess(projectID), msg)
		}

		if err != nil {
			logger.Error("failed to publish organization event", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}
	}

	logger.Info("organization events processed successfully")
	w.WriteHeader(http.StatusAccepted)
}

func (srv *ClientController) UpsertUserScheduledClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if actor == nil {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	projectID := actor.ProjectID
	if projectID == uuid.Nil {
		srv.logger.Warn("project_id is required")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("scheduled", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var req oapi.UpsertUserScheduledRequest
	err = json.Decode(r.Body, &req)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	if req.Identifier == nil || len(*req.Identifier) == 0 {
		srv.logger.Error("at least one identifier is required")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("at least one identifier is required")))
		return
	}
	// Determine schedule type and validate the request.
	scheduleType := "single"
	if req.Interval != nil {
		scheduleType = "recurring"
		if !srv.users.ValidateInterval(ctx, *req.Interval) {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid interval")))
			return
		}
	}

	// For recurring schedules, default start_at to now if not provided.
	// This ensures the scheduler has a valid anchor for computing occurrences.
	if scheduleType == "recurring" && req.StartAt == nil {
		now := time.Now().UTC()
		req.StartAt = &now
	}

	if scheduleType == "single" && req.ScheduledAt == nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("scheduled_at is required for single schedules")))
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("scheduled_name", req.Name))
	logger.Info("upserting user scheduled", zap.String("type", scheduleType))

	var data map[string]any
	if req.Data != nil {
		data = *req.Data
	}

	userIDParams := oapi.ToParams(*req.Identifier)
	msg := schemas.ScheduledMsg{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Name:        req.Name,
		Type:        scheduleType,
		SubjectType: "user",
		Data:        data,
		Identifiers: userIDParams,
		StartAt:     req.StartAt,
		Interval:    req.Interval,
	}

	if req.ScheduledAt != nil {
		msg.ScheduledAt = *req.ScheduledAt
	}

	err = srv.pubsub.Publish(ctx, schemas.ScheduledProcess(projectID), msg)
	if err != nil {
		logger.Error("failed to publish user scheduled", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	logger.Info("user scheduled accepted for processing", zap.Stringer("id", msg.ID))

	var scheduledAt time.Time
	if req.ScheduledAt != nil {
		scheduledAt = *req.ScheduledAt
	} else if req.StartAt != nil {
		scheduledAt = *req.StartAt
	}

	json.Write(w, http.StatusAccepted, oapi.ScheduledAccepted{
		Id:          msg.ID,
		Name:        req.Name,
		ScheduledAt: scheduledAt,
		Data:        req.Data,
	})
}

func (srv *ClientController) DeleteUserScheduledClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if actor == nil {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	projectID := actor.ProjectID
	if projectID == uuid.Nil {
		srv.logger.Warn("project_id is required")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("scheduled", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var req oapi.DeleteUserScheduledRequest
	err = json.Decode(r.Body, &req)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	if req.Identifier == nil || len(*req.Identifier) == 0 {
		srv.logger.Error("at least one identifier is required")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("at least one identifier is required")))
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("scheduled_name", req.Name))
	logger.Info("deleting user scheduled")

	userID, err := srv.users.LookupUserID(ctx, projectID, oapi.ToParams(*req.Identifier))
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

	schedule, err := srv.users.GetScheduleByName(ctx, projectID, req.Name)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("schedule not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("schedule not found")))
		return
	}
	if err != nil {
		logger.Error("failed to get schedule by name", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	err = srv.users.DeleteUserScheduleByScheduleID(ctx, userID, schedule.ID)
	if err != nil {
		logger.Error("failed to delete user schedule", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	logger.Info("user scheduled deleted")
	w.WriteHeader(http.StatusOK)
}

func (srv *ClientController) UpsertOrganizationScheduledClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if actor == nil {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	projectID := actor.ProjectID
	if projectID == uuid.Nil {
		srv.logger.Warn("project_id is required")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("scheduled", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var req oapi.UpsertOrganizationScheduledRequest
	err = json.Decode(r.Body, &req)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("scheduled_name", req.Name))
	logger.Info("upserting organization scheduled")

	orgIdentifiers := oapi.ToParams(req.Identifier)
	orgID, err := srv.users.LookupOrganizationID(ctx, projectID, orgIdentifiers)
	if errors.Is(err, subjects.ErrOrgNotFound) {
		logger.Info("organization not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}
	if err != nil {
		logger.Error("failed to lookup organization", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	// Determine schedule type and validate the request.
	scheduleType := "single"
	if req.Interval != nil {
		scheduleType = "recurring"
		if !srv.users.ValidateInterval(ctx, *req.Interval) {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid interval")))
			return
		}
	}

	// For recurring schedules, default start_at to now if not provided.
	// This ensures the scheduler has a valid anchor for computing occurrences.
	if scheduleType == "recurring" && req.StartAt == nil {
		now := time.Now().UTC()
		req.StartAt = &now
	}

	if scheduleType == "single" && req.ScheduledAt == nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("scheduled_at is required for single schedules")))
		return
	}

	var data map[string]any
	if req.Data != nil {
		data = *req.Data
	}

	msg := schemas.ScheduledMsg{
		ID:             uuid.New(),
		ProjectID:      projectID,
		Name:           req.Name,
		Type:           scheduleType,
		SubjectType:    "organization",
		Data:           data,
		OrganizationID: orgID,
		Identifiers:    orgIdentifiers,
		StartAt:        req.StartAt,
		Interval:       req.Interval,
	}

	if req.ScheduledAt != nil {
		msg.ScheduledAt = *req.ScheduledAt
	}

	err = srv.pubsub.Publish(ctx, schemas.ScheduledProcess(projectID), msg)
	if err != nil {
		logger.Error("failed to publish organization scheduled", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	logger.Info("organization scheduled accepted for processing", zap.Stringer("id", msg.ID))

	var scheduledAt time.Time
	if req.ScheduledAt != nil {
		scheduledAt = *req.ScheduledAt
	} else if req.StartAt != nil {
		scheduledAt = *req.StartAt
	}

	json.Write(w, http.StatusAccepted, oapi.ScheduledAccepted{
		Id:          msg.ID,
		Name:        req.Name,
		ScheduledAt: scheduledAt,
		Data:        req.Data,
	})
}

func (srv *ClientController) DeleteOrganizationScheduledClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if actor == nil {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	projectID := actor.ProjectID
	if projectID == uuid.Nil {
		srv.logger.Warn("project_id is required")
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("scheduled", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var req oapi.DeleteOrganizationScheduledRequest
	err = json.Decode(r.Body, &req)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("scheduled_name", req.Name))
	logger.Info("deleting organization scheduled")

	orgID, err := srv.users.LookupOrganizationID(ctx, projectID, oapi.ToParams(req.Identifier))
	if errors.Is(err, subjects.ErrOrgNotFound) {
		logger.Info("organization not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}
	if err != nil {
		logger.Error("failed to lookup organization", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	schedule, err := srv.users.GetScheduleByName(ctx, projectID, req.Name)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("schedule not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("schedule not found")))
		return
	}
	if err != nil {
		logger.Error("failed to get schedule by name", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	err = srv.users.DeleteOrganizationScheduleByScheduleID(ctx, orgID, schedule.ID)
	if err != nil {
		logger.Error("failed to delete organization schedule", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	logger.Info("organization scheduled deleted")
	w.WriteHeader(http.StatusOK)
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
		Identifier: externalIDRecordsToClientOAPI(org.ExternalIDs),
		Name:       org.Name,
		Data:       data,
		Version:    org.Version,
		CreatedAt:  org.CreatedAt,
		UpdatedAt:  org.UpdatedAt,
	}
}

// externalIDRecordsToClientOAPI converts store ExternalIDRecord slice to client oapi ExternalIDResponse slice.
func externalIDRecordsToClientOAPI(records []subjects.ExternalIDRecord) []oapi.ExternalIDResponse {
	result := make([]oapi.ExternalIDResponse, len(records))
	for i, r := range records {
		result[i] = oapi.ExternalIDResponse{
			Id:         r.ID,
			Source:     r.Source,
			ExternalId: r.ExternalID,
			CreatedAt:  r.CreatedAt,
			UpdatedAt:  r.UpdatedAt,
		}
		if len(r.Metadata) > 0 && string(r.Metadata) != "null" {
			var m map[string]any
			if err := json.Unmarshal(r.Metadata, &m); err == nil {
				result[i].Metadata = &m
			}
		}
	}
	return result
}
