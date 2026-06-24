package v1

import (
	"errors"
	"net/http"

	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/http/controllers/v1/client/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

type OrganizationsController struct {
	*ClientController
}

func NewOrganizationsController(client *ClientController) *OrganizationsController {
	return &OrganizationsController{ClientController: client}
}

func (srv *OrganizationsController) UpsertOrganizationClient(w http.ResponseWriter, r *http.Request, _ oapi.ProjectID) {
	projectID, err := srv.engine.AllowedProject(r.Context(), "organizations", rbac.Create)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	ctx := r.Context()

	if err := auth.RequireCrossSubjectAccess(ctx); err != nil {
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

func (srv *OrganizationsController) DeleteOrganizationClient(w http.ResponseWriter, r *http.Request, _ oapi.ProjectID) {
	projectID, err := srv.engine.AllowedProject(r.Context(), "organizations", rbac.Delete)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	ctx := r.Context()

	if err := auth.RequireCrossSubjectAccess(ctx); err != nil {
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

func (srv *OrganizationsController) AddOrganizationUserClient(w http.ResponseWriter, r *http.Request, _ oapi.ProjectID) {
	projectID, err := srv.engine.AllowedProject(r.Context(), "organizations", rbac.Update)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	ctx := r.Context()

	if err := auth.RequireCrossSubjectAccess(ctx); err != nil {
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

func (srv *OrganizationsController) RemoveOrganizationUserClient(w http.ResponseWriter, r *http.Request, _ oapi.ProjectID) {
	projectID, err := srv.engine.AllowedProject(r.Context(), "organizations", rbac.Update)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	ctx := r.Context()

	if err := auth.RequireCrossSubjectAccess(ctx); err != nil {
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
