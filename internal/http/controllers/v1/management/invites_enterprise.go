//go:build enterprise

package v1

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/rbac/access"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

type InviteController struct {
	logger *zap.Logger
	mgmt   *management.State
	engine *rbac.Engine
	db     *sqlx.DB
}

func NewInviteController(logger *zap.Logger, mgmt *management.State, engine *rbac.Engine, db *sqlx.DB) *InviteController {
	return &InviteController{
		logger: logger,
		mgmt:   mgmt,
		engine: engine,
		db:     db,
	}
}

// myInvitesResponse is the body returned by ListMyInvites. The spec models it as
// an inline object, so it has no dedicated generated type.
type myInvitesResponse struct {
	Results []oapi.ProjectInvite `json:"results"`
}

func isRoleHigher(role1, role2 string) bool {
	roleHierarchy := map[string]int{
		"support": 1,
		"client":  1,
		"editor":  2,
		"admin":   3,
		"owner":   4,
	}

	return roleHierarchy[role1] > roleHierarchy[role2]
}

func (srv *InviteController) CreateProjectInvite(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("invites", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.CreateProjectInviteJSONRequestBody{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	inviteeEmail := strings.ToLower(strings.TrimSpace(string(body.Email)))
	logger := srv.logger.With(zap.String("project_id", projectID.String()), zap.String("email", inviteeEmail))
	logger.Info("creating project invite")

	actor := rbac.FromContext(ctx)
	inviterAdminID, err := uuid.Parse(actor.ID)
	if err != nil {
		logger.Error("actor is not an admin", zap.String("actor_id", actor.ID), zap.Error(err))
		oapi.WriteProblem(w, problem.ErrForbidden(problem.Describe("only admins can create invites")))
		return
	}

	actorAdmin, err := srv.mgmt.GetAdmin(ctx, inviterAdminID)
	if err != nil {
		logger.Error("failed to get inviter admin details", zap.String("admin_id", inviterAdminID.String()), zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if strings.EqualFold(actorAdmin.Email, inviteeEmail) {
		logger.Debug("inviter email matches invitee email, cannot create invite")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("you cannot invite yourself to a project")))
		return
	}

	actorRole := actorAdmin.Role
	if actorRole != "" && isRoleHigher(string(body.Role), actorRole) {
		logger.Debug("invite role is higher than inviter role, cannot create invite", zap.String("invite_role", string(body.Role)), zap.String("inviter_role", actorRole))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("the role assigned by this invite must be equal to or lower than your own global role")))
		return
	}

	project, err := srv.mgmt.GetProject(ctx, projectID, nil)
	if err != nil {
		logger.Error("failed to get project", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Same-org / new-email guard. An invite may target a brand-new email (no
	// admin account yet) or an existing admin that already belongs to the
	// project's organization. Inviting an email that belongs to a *different*
	// organization is rejected until admin↔organization many-to-many membership
	// lands. When the invitee already has an account we denormalize its id so
	// "my invites" can be matched even before the email column is touched.
	var inviteeAdminID *uuid.UUID
	inviteeAdmin, err := srv.mgmt.GetAdminByEmail(ctx, inviteeEmail)
	switch {
	case err == nil:
		if project.OrganizationID == nil || inviteeAdmin.OrganizationID != *project.OrganizationID {
			logger.Debug("invitee belongs to a different organization", zap.String("invitee_admin_id", inviteeAdmin.ID.String()))
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("this email belongs to another organization; multi-organization membership is coming soon")))
			return
		}
		inviteeAdminID = &inviteeAdmin.ID
	case errors.Is(err, sql.ErrNoRows):
		// Brand-new invitee — resolved by email when they sign up.
	default:
		logger.Error("failed to look up invitee admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	expiresIn := "24h"
	if body.ExpiresIn != nil {
		expiresIn = *body.ExpiresIn
	}

	invite, err := srv.mgmt.CreateProjectInvite(ctx, projectID, inviterAdminID, inviteeEmail, inviteeAdminID, body.Role, expiresIn)
	if err != nil {
		logger.Error("failed to create project invite", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("created project invite", zap.String("invite_id", invite.ID.String()))
	json.Write(w, http.StatusCreated, invite.OAPI())
}

// ListMyInvites returns the pending invites addressed to the authenticated
// admin's email. This is how an invitee discovers invites — there is no token,
// the invite is bound to their (IdP-verified) email.
func (srv *InviteController) ListMyInvites(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)

	adminID, err := uuid.Parse(actor.ID)
	if err != nil {
		srv.logger.Error("actor is not an admin", zap.String("actor_id", actor.ID), zap.Error(err))
		oapi.WriteProblem(w, problem.ErrForbidden(problem.Describe("only admins have invites")))
		return
	}

	admin, err := srv.mgmt.GetAdmin(ctx, adminID)
	if err != nil {
		srv.logger.Error("failed to get admin", zap.String("admin_id", adminID.String()), zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	invites, err := srv.mgmt.ListInvitesForEmail(ctx, admin.Email)
	if err != nil {
		srv.logger.Error("failed to list invites for email", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	results := make([]oapi.ProjectInvite, len(invites))
	for i := range invites {
		results[i] = invites[i].OAPI()
	}

	json.Write(w, http.StatusOK, myInvitesResponse{Results: results})
}

func (srv *InviteController) AcceptProjectInvite(w http.ResponseWriter, r *http.Request, inviteID uuid.UUID) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)

	adminID, err := uuid.Parse(actor.ID)
	if err != nil {
		srv.logger.Error("actor is not an admin", zap.String("actor_id", actor.ID), zap.Error(err))
		oapi.WriteProblem(w, problem.ErrForbidden(problem.Describe("only admins can accept invites")))
		return
	}

	logger := srv.logger.With(zap.String("invite_id", inviteID.String()), zap.String("admin_id", adminID.String()))

	admin, err := srv.mgmt.GetAdmin(ctx, adminID)
	if err != nil {
		logger.Error("failed to get admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	invite, err := srv.mgmt.GetInviteByID(ctx, inviteID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("invite not found")))
		} else {
			logger.Error("failed to get invite", zap.Error(err))
			oapi.WriteProblem(w, err)
		}
		return
	}

	// The invite is bound to an email; only the admin who owns that (verified)
	// email may accept it.
	if !strings.EqualFold(admin.Email, invite.InviteeEmail) {
		logger.Debug("admin email does not match invitee email", zap.String("invitee_email", invite.InviteeEmail))
		oapi.WriteProblem(w, problem.ErrForbidden(problem.Describe("this invite was not sent to your account")))
		return
	}

	switch {
	case invite.RevokedAt != nil:
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("this invite has been revoked")))
		return
	case invite.AcceptedAt != nil:
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("this invite has already been accepted")))
		return
	case invite.ExpiresAt.Before(time.Now()):
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("this invite has expired")))
		return
	}

	tx, err := srv.db.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	defer tx.Rollback() //nolint:errcheck

	managementStore := management.NewState(tx)

	existingProjectAdmin, err := managementStore.GetProjectAdmin(ctx, invite.ProjectID, adminID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		logger.Error("failed to check existing project admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	roleUpgraded := false
	if existingProjectAdmin != nil {
		if isRoleHigher(invite.Role, existingProjectAdmin.Role) {
			err = managementStore.UpdateProjectAdminRole(ctx, invite.ProjectID, adminID, invite.Role)
			if err != nil {
				logger.Error("failed to update admin project role", zap.Error(err))
				oapi.WriteProblem(w, err)
				return
			}
			roleUpgraded = true
		}
	} else {
		err = managementStore.AddAdminToProject(ctx, invite.ProjectID, adminID, invite.Role)
		if err != nil {
			logger.Error("failed to add admin to project", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
	}

	// TODO: once admin↔organization many-to-many membership lands, also add the
	// admin to the organization that owns this project.

	if _, err = managementStore.AcceptProjectInvite(ctx, inviteID); err != nil {
		logger.Error("failed to accept project invite", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	project, err := managementStore.GetProject(ctx, invite.ProjectID, &adminID)
	if err != nil {
		logger.Error("failed to load project after accept", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if err = tx.Commit(); err != nil {
		logger.Error("failed to commit transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// RBAC tuples are written to OpenFGA, which is not part of the Postgres
	// transaction, so they are applied only after the membership is durably
	// committed. This avoids granting access for a membership that rolled back.
	if roleUpgraded {
		oldTuples := access.ProjectRoleTuples(adminID, invite.ProjectID, existingProjectAdmin.Role)
		if err = srv.engine.DeleteTuples(ctx, oldTuples); err != nil {
			logger.Error("failed to delete old RBAC tuples", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to update project role")))
			return
		}
	}

	projectTuples := access.ProjectRoleTuples(adminID, invite.ProjectID, invite.Role)
	if err = srv.engine.WriteTuples(ctx, projectTuples); err != nil {
		logger.Error("failed to write RBAC tuples for project admin", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to assign project role")))
		return
	}

	logger.Info("accepted project invite and added admin to project", zap.String("project_id", invite.ProjectID.String()))
	json.Write(w, http.StatusOK, project.OAPI())
}

func (srv *InviteController) RevokeProjectInvite(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, inviteID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("invites", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	invite, err := srv.mgmt.GetInviteByID(ctx, inviteID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("invite not found")))
		} else {
			srv.logger.Error("failed to get invite", zap.Error(err))
			oapi.WriteProblem(w, err)
		}
		return
	}

	// The permission was checked against projectID; make sure the invite really
	// belongs to that project before mutating it.
	if invite.ProjectID != projectID {
		srv.logger.Warn("invite project mismatch on revoke", zap.String("invite_id", inviteID.String()), zap.String("project_id", projectID.String()))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("invite not found")))
		return
	}

	if _, err = srv.mgmt.RevokeProjectInvite(ctx, inviteID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("invite not found or already resolved")))
		} else {
			srv.logger.Error("failed to revoke invite", zap.Error(err))
			oapi.WriteProblem(w, err)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (srv *InviteController) ListProjectInvites(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListProjectInvitesParams) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("invites", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("listing project invites")

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	var expiresBefore, expiresAfter *string
	if params.ExpiresBefore != nil {
		s := params.ExpiresBefore.Time.Format(time.RFC3339)
		expiresBefore = &s
	}
	if params.ExpiresAfter != nil {
		s := params.ExpiresAfter.Time.Format(time.RFC3339)
		expiresAfter = &s
	}

	invites, total, err := srv.mgmt.ListProjectInvites(ctx, projectID, pagination, params.Search.ToString(), params.Role, params.Status, expiresBefore, expiresAfter, params.InviterAdminId)
	if err != nil {
		logger.Error("failed to list project invites", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("listed project invites", zap.Int("count", len(invites)))
	response := make([]oapi.ProjectInvite, len(invites))
	for i := range invites {
		response[i] = invites[i].OAPI()
	}

	json.Write(w, http.StatusOK, oapi.ProjectInviteListResponse{
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		Results: response,
	})
}
