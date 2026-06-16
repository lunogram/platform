package v1

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	httpparams "github.com/lunogram/platform/internal/http/params"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/consumer"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/lunogram/platform/pkg/modules"
	"go.uber.org/zap"
)

func NewOrganizationsController(logger *zap.Logger, usersDB *sqlx.DB, mgmt *management.State, pub pubsub.Publisher, engine *rbac.Engine) *OrganizationsController {
	return &OrganizationsController{
		logger: logger,
		db:     usersDB,
		mgmt:   mgmt,
		orgs:   subjects.NewState(usersDB, logger),
		events: subjects.NewEventsStore(usersDB),
		pubsub: pub,
		engine: engine,
	}
}

type OrganizationsController struct {
	logger *zap.Logger
	db     *sqlx.DB
	mgmt   *management.State
	orgs   *subjects.State
	events *subjects.EventsStore
	pubsub pubsub.Publisher
	engine *rbac.Engine
}

func (srv *OrganizationsController) ListOrganizations(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListOrganizationsParams) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("organizations", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.String("project_id", projectID.String()))

	search := params.Search.ToString()
	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	logger.Info("listing subject organizations", zap.Int("limit", pagination.Limit), zap.Int("offset", pagination.Offset))

	orgs, total, err := srv.orgs.ListOrganizations(ctx, projectID, pagination, search)
	if err != nil {
		logger.Error("failed to list organizations", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("organizations listed", zap.Int("total", total), zap.Int("count", len(orgs)))

	response := oapi.OrganizationList{
		Results: orgs.OAPI(),
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
	}

	json.Write(w, http.StatusOK, response)
}

func (srv *OrganizationsController) UpsertOrganization(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("organizations", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.UpsertOrganization{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	if len(body.Identifier) == 0 {
		srv.logger.Error("at least one identifier is required")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("at least one identifier is required")))
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
	)

	logger.Info("upserting subject organization")

	var data map[string]any
	if body.Data != nil {
		data = *body.Data
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
		Identifiers: oapi.ToParams(body.Identifier),
		Name:        body.Name,
		Data:        data,
	}

	orgID, err := orgsStore.UpsertOrganization(ctx, projectID, params)
	if errors.Is(err, subjects.ErrOrgIdentifierBelongsToOther) {
		logger.Info("identifier belongs to another organization", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrConflict(problem.Describe("one or more identifiers already belong to a different organization")))
		return
	}
	if errors.Is(err, subjects.ErrOrgConflictingIdentifiers) {
		logger.Info("conflicting identifiers", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrConflict(problem.Describe("identifiers resolve to different existing organizations")))
		return
	}
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
		Identifiers: org.ExternalIDs.Params(),
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
	json.Write(w, http.StatusOK, org.OAPI())
}

func (srv *OrganizationsController) GetOrganization(w http.ResponseWriter, r *http.Request, projectID, organizationID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("organizations", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("organization_id", organizationID.String()),
	)

	logger.Info("getting subject organization")

	org, err := srv.orgs.GetOrganization(ctx, projectID, organizationID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("organization not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get organization", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("organization retrieved")
	json.Write(w, http.StatusOK, org.OAPI())
}

func (srv *OrganizationsController) UpdateOrganization(w http.ResponseWriter, r *http.Request, projectID, organizationID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("organizations", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.orgs.GetOrganization(ctx, projectID, organizationID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("organization not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get organization", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.UpdateOrganization{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("organization_id", organizationID.String()),
	)

	logger.Info("updating subject organization")

	var data map[string]any
	if body.Data != nil {
		err = json.Unmarshal(*body.Data, &data)
		if err != nil {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("data must be a JSON object")))
			return
		}
	}

	tx, err := srv.db.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	defer tx.Rollback() //nolint:errcheck
	organizations := subjects.NewOrganizationsStore(tx)

	update := subjects.OrganizationUpdate{
		Name: body.Name,
		Data: body.Data,
	}

	err = organizations.UpdateOrganization(ctx, projectID, organizationID, update)
	if err != nil {
		logger.Error("failed to update organization", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	updated, err := organizations.GetOrganization(ctx, projectID, organizationID)
	if err != nil {
		logger.Error("failed to get updated organization", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Publish to pubsub for recomputation and schema extraction
	msg := schemas.Organization{
		ID:          updated.ID,
		ProjectID:   projectID,
		Identifiers: updated.ExternalIDs.Params(),
		Name:        updated.Name,
		Data:        data,
		Version:     updated.Version,
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

	logger.Info("organization updated")
	json.Write(w, http.StatusOK, updated.OAPI())
}

func (srv *OrganizationsController) DeleteOrganization(w http.ResponseWriter, r *http.Request, projectID, organizationID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("organizations", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.orgs.GetOrganization(ctx, projectID, organizationID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("organization not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get organization", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("organization_id", organizationID.String()),
	)

	logger.Info("deleting subject organization")

	err = srv.orgs.DeleteOrganization(ctx, projectID, organizationID)
	if err != nil {
		logger.Error("failed to delete organization", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("organization deleted")
	w.WriteHeader(http.StatusNoContent)
}

func (srv *OrganizationsController) ListOrganizationMembers(w http.ResponseWriter, r *http.Request, projectID, organizationID uuid.UUID, params oapi.ListOrganizationMembersParams) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("organizations", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.orgs.GetOrganization(ctx, projectID, organizationID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("organization not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get organization", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("organization_id", organizationID.String()),
	)

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	logger.Info("listing organization users", zap.Int("limit", pagination.Limit), zap.Int("offset", pagination.Offset))

	members, total, err := srv.orgs.ListOrganizationMembers(ctx, projectID, organizationID, pagination)
	if err != nil {
		logger.Error("failed to list organization members", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("organization users listed", zap.Int("total", total), zap.Int("count", len(members)))

	response := oapi.OrganizationMemberList{
		Results: members.OAPI(),
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
	}

	json.Write(w, http.StatusOK, response)
}

func (srv *OrganizationsController) AddOrganizationMember(w http.ResponseWriter, r *http.Request, projectID, organizationID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("organizations", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	org, err := srv.orgs.GetOrganization(ctx, projectID, organizationID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("organization not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get organization", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.AddOrganizationMember{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("organization_id", organizationID.String()),
		zap.String("user_id", body.UserId.String()),
	)

	logger.Info("adding user to organization")

	// Verify user exists
	_, err = srv.orgs.GetUser(ctx, projectID, body.UserId)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("user not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	var data map[string]any
	if body.Data != nil {
		data = *body.Data
	}

	tx, err := srv.db.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	defer tx.Rollback() //nolint:errcheck
	orgsStore := subjects.NewOrganizationsStore(tx)

	orgUser, err := orgsStore.UpsertAndGetOrganizationMember(ctx, organizationID, body.UserId, data)
	if err != nil {
		logger.Error("failed to add user to organization", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Publish to pubsub for schema extraction and event firing
	// Version 0 means new membership (will fire organization.user.added)
	// Version > 0 means existing membership (will fire organization.user.updated)
	msg := schemas.OrganizationUser{
		OrganizationID:          organizationID,
		OrganizationIdentifiers: org.ExternalIDs.Params(),
		UserID:                  body.UserId,
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

func (srv *OrganizationsController) RemoveOrganizationMember(w http.ResponseWriter, r *http.Request, projectID, organizationID, userID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("organizations", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.orgs.GetOrganization(ctx, projectID, organizationID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("organization not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get organization", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("organization_id", organizationID.String()),
		zap.String("user_id", userID.String()),
	)

	logger.Info("removing user from organization")

	err = srv.orgs.RemoveUserFromOrganization(ctx, organizationID, userID)
	if err != nil {
		logger.Error("failed to remove user from organization", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("user removed from organization")
	w.WriteHeader(http.StatusNoContent)
}

func (srv *OrganizationsController) GetOrganizationEvents(w http.ResponseWriter, r *http.Request, projectID, organizationID uuid.UUID, params oapi.GetOrganizationEventsParams) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("events", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.orgs.GetOrganization(ctx, projectID, organizationID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("organization not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get organization", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("organization_id", organizationID.String()),
	)

	search := params.Search.ToString()
	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	logger.Info("listing organization events", zap.Int("limit", pagination.Limit), zap.Int("offset", pagination.Offset))

	events, total, err := srv.orgs.ListOrganizationEvents(ctx, projectID, organizationID, pagination, search)
	if err != nil {
		logger.Error("failed to list organization events", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("organization events listed", zap.Int("total", total), zap.Int("count", len(events)))

	response := oapi.OrganizationEventList{
		Results: events.OAPI(),
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
	}

	json.Write(w, http.StatusOK, response)
}

func (srv *OrganizationsController) GetOrganizationInboxMessages(w http.ResponseWriter, r *http.Request, projectID, organizationID uuid.UUID, params oapi.GetOrganizationInboxMessagesParams) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("inbox", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.orgs.GetOrganization(ctx, projectID, organizationID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("organization not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}
	if err != nil {
		srv.logger.Error("failed to get organization", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	filter := subjects.InboxListFilter{
		MessageSource:    params.MessageSource,
		Priority:         params.Priority,
		Search:           params.Search.ToString(),
		IncludeArchived:  params.IncludeArchived != nil && *params.IncludeArchived,
		IncludeScheduled: params.IncludeScheduled != nil && *params.IncludeScheduled,
	}
	if params.Status != nil {
		filter.Status = string(*params.Status)
	}
	if params.Channel != nil {
		filter.Channel = modules.Channel(*params.Channel)
	}
	if tags := httpparams.Split(ptr.From(params.Tags)); len(tags) > 0 {
		filter.Tags = tags
	}

	pagination := store.Pagination{Limit: params.Limit.ToInt(), Offset: params.Offset.ToInt()}
	messages, total, err := srv.orgs.ListOrganizationInboxMessages(ctx, projectID, organizationID, pagination, filter)
	if err != nil {
		srv.logger.Error("failed to list organization inbox messages", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	json.Write(w, http.StatusOK, oapi.InboxMessageList{Results: messages.OAPI(), Total: total, Limit: pagination.Limit, Offset: pagination.Offset})
}

func (srv *OrganizationsController) CreateOrganizationInboxMessage(w http.ResponseWriter, r *http.Request, projectID, organizationID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("inbox", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.orgs.GetOrganization(ctx, projectID, organizationID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("organization not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}
	if err != nil {
		srv.logger.Error("failed to get organization", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	var body oapi.CreateOrganizationInboxMessageJSONRequestBody
	if err := json.Decode(r.Body, &body); err != nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	tx, err := srv.db.BeginTxx(ctx, nil)
	if err != nil {
		srv.logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	defer tx.Rollback() //nolint:errcheck

	inbox := subjects.NewInboxStore(tx)
	params, err := inboxMessageParams(body)
	if err != nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid message data")))
		return
	}
	message, err := inbox.CreateOrganizationInboxMessage(ctx, projectID, organizationID, params)
	if errors.Is(err, sql.ErrNoRows) {
		oapi.WriteProblem(w, problem.ErrConflict(problem.Describe("inbox message already exists")))
		return
	}
	if err != nil {
		srv.logger.Error("failed to create organization inbox message", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if err := tx.Commit(); err != nil {
		srv.logger.Error("failed to commit organization inbox message", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if message.IsDue() {
		if err := srv.pubsub.Publish(ctx, schemas.OrganizationInboxProcess(projectID), schemas.InboxMessage{
			ProjectID: projectID,
			MessageID: message.ID,
			SubjectID: organizationID,
			Channel:   string(message.Channel),
		}); err != nil {
			srv.logger.Error("failed to publish organization inbox message", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}
	}

	json.Write(w, http.StatusCreated, message.OAPI())
}

func (srv *OrganizationsController) ReadOrganizationInboxMessage(w http.ResponseWriter, r *http.Request, projectID, organizationID, messageID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("inbox", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	tx, err := srv.db.BeginTxx(ctx, nil)
	if err != nil {
		srv.logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	defer tx.Rollback() //nolint:errcheck

	inbox := subjects.NewInboxStore(tx)
	message, transitioned, err := inbox.ReadOrganizationInboxMessage(ctx, projectID, organizationID, messageID)
	if errors.Is(err, sql.ErrNoRows) {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("inbox message not found")))
		return
	}
	if err != nil {
		srv.logger.Error("failed to read organization inbox message", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		srv.logger.Error("failed to commit organization inbox message update", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	if transitioned {
		if err := consumer.PublishInboxLifecycleEvent(ctx, srv.pubsub, message, schemas.EventInboxMessageRead); err != nil {
			srv.logger.Error("failed to publish organization inbox event", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}
	}
	json.Write(w, http.StatusOK, message.OAPI())
}

func (srv *OrganizationsController) ArchiveOrganizationInboxMessage(w http.ResponseWriter, r *http.Request, projectID, organizationID, messageID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("inbox", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	tx, err := srv.db.BeginTxx(ctx, nil)
	if err != nil {
		srv.logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	defer tx.Rollback() //nolint:errcheck

	inbox := subjects.NewInboxStore(tx)
	message, transitioned, err := inbox.ArchiveOrganizationInboxMessage(ctx, projectID, organizationID, messageID)
	if errors.Is(err, sql.ErrNoRows) {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("inbox message not found")))
		return
	}
	if err != nil {
		srv.logger.Error("failed to archive organization inbox message", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		srv.logger.Error("failed to commit organization inbox message update", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	if transitioned {
		if err := consumer.PublishInboxLifecycleEvent(ctx, srv.pubsub, message, schemas.EventInboxMessageArchived); err != nil {
			srv.logger.Error("failed to publish organization inbox event", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}
	}
	json.Write(w, http.StatusOK, message.OAPI())
}

func (srv *OrganizationsController) UnarchiveOrganizationInboxMessage(w http.ResponseWriter, r *http.Request, projectID, organizationID, messageID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("inbox", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	tx, err := srv.db.BeginTxx(ctx, nil)
	if err != nil {
		srv.logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	defer tx.Rollback() //nolint:errcheck

	inbox := subjects.NewInboxStore(tx)
	message, transitioned, err := inbox.UnarchiveOrganizationInboxMessage(ctx, projectID, organizationID, messageID)
	if errors.Is(err, sql.ErrNoRows) {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("inbox message not found")))
		return
	}
	if err != nil {
		srv.logger.Error("failed to unarchive organization inbox message", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		srv.logger.Error("failed to commit organization inbox message update", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	if transitioned {
		if err := consumer.PublishInboxLifecycleEvent(ctx, srv.pubsub, message, schemas.EventInboxMessageUnarchived); err != nil {
			srv.logger.Error("failed to publish organization inbox event", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}
	}
	json.Write(w, http.StatusOK, message.OAPI())
}

func (srv *OrganizationsController) UnreadOrganizationInboxMessage(w http.ResponseWriter, r *http.Request, projectID, organizationID, messageID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("inbox", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	tx, err := srv.db.BeginTxx(ctx, nil)
	if err != nil {
		srv.logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	defer tx.Rollback() //nolint:errcheck

	inbox := subjects.NewInboxStore(tx)
	message, transitioned, err := inbox.UnreadOrganizationInboxMessage(ctx, projectID, organizationID, messageID)
	if errors.Is(err, sql.ErrNoRows) {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("inbox message not found")))
		return
	}
	if err != nil {
		srv.logger.Error("failed to mark organization inbox message as unread", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	if err := tx.Commit(); err != nil {
		srv.logger.Error("failed to commit organization inbox message update", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	if transitioned {
		if err := consumer.PublishInboxLifecycleEvent(ctx, srv.pubsub, message, schemas.EventInboxMessageUnread); err != nil {
			srv.logger.Error("failed to publish organization inbox event", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}
	}
	json.Write(w, http.StatusOK, message.OAPI())
}

func (srv *OrganizationsController) RescheduleOrganizationInboxMessage(w http.ResponseWriter, r *http.Request, projectID, organizationID, messageID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("inbox", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}
	if err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("scheduled", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var body oapi.RescheduleOrganizationInboxMessageJSONRequestBody
	if err := json.Decode(r.Body, &body); err != nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	message, err := srv.orgs.UpdateOrganizationInboxMessageScheduledAt(ctx, projectID, organizationID, messageID, body.ScheduledAt)
	if errors.Is(err, sql.ErrNoRows) {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("inbox message not found")))
		return
	}
	if errors.Is(err, subjects.ErrInboxMessageAlreadyDue) {
		oapi.WriteProblem(w, problem.ErrConflict(problem.Describe("inbox message is already due and cannot be rescheduled")))
		return
	}
	if err != nil {
		srv.logger.Error("failed to reschedule organization inbox message", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	json.Write(w, http.StatusOK, message.OAPI())
}

func (srv *OrganizationsController) CreateOrganizationEvent(w http.ResponseWriter, r *http.Request, projectID, organizationID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("events", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.orgs.GetOrganization(ctx, projectID, organizationID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("organization not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get organization", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	var body oapi.CreateOrganizationEventJSONRequestBody
	err = json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	if body.Name == "" {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("event name is required")))
		return
	}

	msg := schemas.OrganizationEvent{
		ProjectID:      projectID,
		OrganizationID: organizationID,
		Name:           body.Name,
		Data:           nil,
	}
	if body.Data != nil {
		msg.Data = *body.Data
	}

	err = srv.pubsub.Publish(ctx, schemas.OrganizationEventsProcess(projectID), msg)
	if err != nil {
		srv.logger.Error("failed to publish organization event", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (srv *OrganizationsController) ListOrganizationSchemas(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("organizations", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.String("project_id", projectID.String()))
	logger.Info("listing organization schemas")

	schemas, err := srv.orgs.ListOrganizationSchemas(ctx, projectID)
	if err != nil {
		logger.Error("failed to list organization schemas", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("organization schemas listed", zap.Int("count", len(schemas)))

	results := make([]oapi.SchemaPath, len(schemas))
	for i, schema := range schemas {
		results[i] = oapi.SchemaPath{
			Path:  schema.Path,
			Types: []string(schema.Types),
		}
	}

	response := struct {
		Results []oapi.SchemaPath `json:"results"`
	}{
		Results: results,
	}

	json.Write(w, http.StatusOK, response)
}

func (srv *OrganizationsController) ListOrganizationMemberSchemas(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("organizations", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.String("project_id", projectID.String()))
	logger.Info("listing organization user schemas")

	schemas, err := srv.orgs.ListOrganizationUserSchemas(ctx, projectID)
	if err != nil {
		logger.Error("failed to list organization user schemas", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("organization user schemas listed", zap.Int("count", len(schemas)))

	results := make([]oapi.SchemaPath, len(schemas))
	for i, schema := range schemas {
		results[i] = oapi.SchemaPath{
			Path:  schema.Path,
			Types: []string(schema.Types),
		}
	}

	response := struct {
		Results []oapi.SchemaPath `json:"results"`
	}{
		Results: results,
	}

	json.Write(w, http.StatusOK, response)
}

func (srv *OrganizationsController) ListOrganizationEventSchemas(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("events", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.String("project_id", projectID.String()))
	logger.Info("listing organization event schemas")

	events, err := srv.events.ListEventSchemas(ctx, projectID, subjects.SubjectTypeOrganization)
	if err != nil {
		logger.Error("failed to list organization event schemas", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("organization event schemas listed", zap.Int("count", len(events)))

	results := make([]oapi.EventWithSchema, len(events))
	for i, event := range events {
		schema := make([]oapi.SchemaPath, len(event.Schema))
		for j, s := range event.Schema {
			schema[j] = oapi.SchemaPath{
				Path:  s.Path,
				Types: []string(s.Types),
			}
		}

		results[i] = oapi.EventWithSchema{
			Id:     event.ID,
			Name:   event.Name,
			Schema: schema,
		}
	}

	json.Write(w, http.StatusOK, oapi.EventListResponse{
		Results: results,
	})
}

func (srv *OrganizationsController) DeleteOrganizationEventSchema(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, eventID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("events", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("event_id", eventID.String()),
	)

	logger.Info("deleting organization event schema")

	err = srv.events.DeleteEvent(ctx, projectID, eventID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("event not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("event not found")))
		return
	}

	if err != nil {
		logger.Error("failed to delete event", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("organization event schema deleted")
	w.WriteHeader(http.StatusNoContent)
}

func (srv *OrganizationsController) DeleteOrganizationExternalID(w http.ResponseWriter, r *http.Request, projectID, organizationID, identifierID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("organizations", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("organization_id", organizationID.String()),
		zap.String("identifier_id", identifierID.String()),
	)

	// Verify the organization exists in this project.
	_, err = srv.orgs.GetOrganization(ctx, projectID, organizationID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("organization not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}
	if err != nil {
		logger.Error("failed to get organization", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	logger.Info("deleting organization external identifier")

	err = srv.orgs.DeleteOrgExternalIDByID(ctx, organizationID, identifierID)
	if errors.Is(err, subjects.ErrOrgLastIdentifier) {
		logger.Info("cannot delete last identifier")
		oapi.WriteProblem(w, problem.ErrConflict(problem.Describe("cannot delete the last remaining identifier")))
		return
	}
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("identifier not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("identifier not found")))
		return
	}
	if err != nil {
		logger.Error("failed to delete organization external identifier", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	logger.Info("organization external identifier deleted")
	w.WriteHeader(http.StatusNoContent)
}
