//go:build enterprise

package v1

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
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

// projectRoleRank ranks the project-level roles for least-privilege comparisons.
// Project roles form a strict hierarchy: support < client < editor < admin.
// "owner" is an *organization*-level role, not a project role (the OpenFGA
// "project" type has no "owner" relation), so it is deliberately absent here —
// an unknown role ranks 0 and can never out-rank a real project role.
var projectRoleRank = map[string]int{
	"support": 1,
	"client":  2,
	"editor":  3,
	"admin":   4,
}

func isRoleHigher(role1, role2 string) bool {
	return projectRoleRank[role1] > projectRoleRank[role2]
}

// effectiveProjectRole resolves the inviter's effective role *within a specific
// project* for least-privilege checks. The actor's global organization role
// ("owner"/"admin") grants project "admin" by inheritance (see the OpenFGA
// "project" type, where project admin = ttu organization owner/admin), so an
// org owner/admin may assign any project role. Otherwise the explicit
// project_admins role is used; an actor with neither has no authority and
// resolves to "" (rank 0), which cannot out-rank any real role.
func (srv *InviteController) effectiveProjectRole(ctx context.Context, orgRole string, projectID, adminID uuid.UUID) (string, error) {
	if orgRole == rbac.OrganizationOwner || orgRole == rbac.OrganizationAdmin {
		return rbac.ProjectAdmin, nil
	}

	role, err := srv.mgmt.GetProjectRole(ctx, projectID, adminID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return role, nil
}

const (
	// defaultInviteTTL is used when the caller omits expires_in.
	defaultInviteTTL = 24 * time.Hour
	// maxInviteTTL caps how far in the future an invite may be valid. Without an
	// upper bound a caller could mint an effectively permanent invite.
	maxInviteTTL = 30 * 24 * time.Hour
)

// parseInviteTTL parses the user-supplied expires_in value into a duration. It
// accepts Go-style durations ("24h", "90m", "30s") plus a "<n>d" day form
// ("7d") that time.ParseDuration does not understand. The result must be
// strictly positive and is clamped to maxInviteTTL. A malformed value returns
// an error so the handler can answer 400 rather than letting Postgres reject a
// bad interval with a 500.
func parseInviteTTL(raw string) (time.Duration, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, errors.New("expires_in must not be empty")
	}

	var d time.Duration
	if rest, ok := strings.CutSuffix(s, "d"); ok {
		days, err := strconv.Atoi(strings.TrimSpace(rest))
		if err != nil {
			return 0, fmt.Errorf("invalid expires_in %q: %w", raw, err)
		}
		d = time.Duration(days) * 24 * time.Hour
	} else {
		parsed, err := time.ParseDuration(s)
		if err != nil {
			return 0, fmt.Errorf("invalid expires_in %q: %w", raw, err)
		}
		d = parsed
	}

	if d <= 0 {
		return 0, fmt.Errorf("expires_in must be positive, got %q", raw)
	}
	if d > maxInviteTTL {
		d = maxInviteTTL
	}
	return d, nil
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

	// Least-privilege ceiling: an inviter may only grant a role at or below their
	// own *project-scoped* role. Evaluating the project role (rather than the
	// near-uniform global admin role) prevents a low-privilege project member
	// from minting a higher-privilege invite.
	actorProjectRole, err := srv.effectiveProjectRole(ctx, actorAdmin.Role, projectID, inviterAdminID)
	if err != nil {
		logger.Error("failed to resolve inviter project role", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	if isRoleHigher(string(body.Role), actorProjectRole) {
		logger.Debug("invite role is higher than inviter project role, cannot create invite", zap.String("invite_role", string(body.Role)), zap.String("inviter_project_role", actorProjectRole))
		oapi.WriteProblem(w, problem.ErrForbidden(problem.Describe("the role assigned by this invite must be equal to or lower than your own role in this project")))
		return
	}

	// The invitee may be a brand-new email (no admin account yet) or an existing
	// admin — possibly from another organization, who will be added as a member
	// of this project's organization when they accept. When the invitee already
	// has an account we denormalize its id so "my invites" can be matched even
	// before they next sign in.
	var inviteeAdminID *uuid.UUID
	inviteeAdmin, err := srv.mgmt.GetAdminByEmail(ctx, inviteeEmail)
	switch {
	case err == nil:
		// The invitee may belong to a different organization; accepting the invite
		// adds them as a member of this project's organization (see AcceptProjectInvite).
		inviteeAdminID = &inviteeAdmin.ID
	case errors.Is(err, sql.ErrNoRows):
		// Brand-new invitee — resolved by email when they sign up.
	default:
		logger.Error("failed to look up invitee admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	ttl := defaultInviteTTL
	if body.ExpiresIn != nil {
		ttl, err = parseInviteTTL(*body.ExpiresIn)
		if err != nil {
			logger.Debug("invalid expires_in", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("expires_in must be a positive duration such as \"24h\" or \"7d\"")))
			return
		}
	}

	invite, err := srv.mgmt.CreateProjectInvite(ctx, projectID, inviterAdminID, inviteeEmail, inviteeAdminID, body.Role, ttl)
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
	if actor == nil {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

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
	if actor == nil {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

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

	// Revoked and expired invites are terminal — they can never be accepted.
	// An *already accepted* invite is NOT rejected here: because the OpenFGA
	// tuple write happens after the Postgres commit (the two stores cannot share
	// a transaction), a tuple-write failure on the first attempt would leave the
	// invitee as a project member in Postgres but without RBAC access. Letting
	// the rightful invitee re-accept reconciles that state by (idempotently)
	// re-writing the tuples below.
	switch {
	case invite.RevokedAt != nil:
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("this invite has been revoked")))
		return
	case invite.AcceptedAt == nil && invite.ExpiresAt.Before(time.Now()):
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

	var previousRole string
	roleUpgraded := false
	if existingProjectAdmin != nil {
		previousRole = existingProjectAdmin.Role
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

	// Mark the invite accepted. The guarded UPDATE is the real arbiter under
	// concurrency: it only matches a pending, unexpired invite. sql.ErrNoRows
	// therefore means the invite was already accepted, or was revoked/expired in
	// the window between our read and this write. When the invite was already
	// accepted (by us, the rightful invitee) we proceed to reconcile RBAC tuples
	// rather than erroring. Otherwise it lost a race to revoke/expire.
	if _, err = managementStore.AcceptProjectInvite(ctx, inviteID); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			logger.Error("failed to accept project invite", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
		if invite.AcceptedAt == nil {
			// We read it as pending but the UPDATE matched nothing and it was not
			// already accepted on read — it was concurrently revoked or expired.
			logger.Debug("invite no longer acceptable (concurrent revoke/expire)")
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("this invite is no longer valid")))
			return
		}
		// Already accepted by the rightful invitee: reconcile path, fall through.
	}

	project, err := managementStore.GetProject(ctx, invite.ProjectID, &adminID)
	if err != nil {
		logger.Error("failed to load project after accept", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Ensure the admin is a member of the organization that owns this project.
	// This is the cross-organization case: accepting an invite into another
	// org's project makes the admin a base member of that organization so it
	// appears in their organization switcher and org-scoped reads resolve. An
	// existing membership (e.g. their home org) is left untouched so we never
	// downgrade an owner to a member.
	addedToOrg := false
	var orgID uuid.UUID
	if project.OrganizationID != nil {
		orgID = *project.OrganizationID
		isMember, err := managementStore.IsMember(ctx, orgID, adminID)
		if err != nil {
			logger.Error("failed to check organization membership", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
		if !isMember {
			if err := managementStore.AddMember(ctx, orgID, adminID, "member"); err != nil {
				logger.Error("failed to add organization membership", zap.Error(err))
				oapi.WriteProblem(w, err)
				return
			}
			addedToOrg = true
		}
	}

	if err = tx.Commit(); err != nil {
		logger.Error("failed to commit transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// RBAC tuples are written to OpenFGA, which is not part of the Postgres
	// transaction, so they are applied only after the membership is durably
	// committed. This avoids granting access for a membership that rolled back.
	// On a role upgrade the stale lower-role tuple is removed first.
	if roleUpgraded && previousRole != invite.Role {
		oldTuples := access.ProjectRoleTuples(adminID, invite.ProjectID, previousRole)
		if err = srv.engine.DeleteTuples(ctx, oldTuples); err != nil {
			logger.Error("failed to delete old RBAC tuples", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to update project role")))
			return
		}
	}

	// WriteTuplesIfMissing makes the grant idempotent so a re-accept (reconcile)
	// does not fail on an already-present tuple, while still repairing a
	// membership whose tuples were never written on a previous attempt.
	projectTuples := access.ProjectRoleTuples(adminID, invite.ProjectID, invite.Role)
	if err = srv.engine.WriteTuplesIfMissing(ctx, projectTuples); err != nil {
		logger.Error("failed to write RBAC tuples for project admin", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to assign project role")))
		return
	}

	if addedToOrg {
		if err = srv.engine.WriteTuples(ctx, access.OrganizationRoleTuples(adminID, orgID, "member")); err != nil {
			logger.Error("failed to write organization RBAC tuples", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to assign organization membership")))
			return
		}
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
