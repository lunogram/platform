package v1

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

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

func NewAdminsController(logger *zap.Logger, db *sqlx.DB, engine *rbac.Engine) *AdminsController {
	return &AdminsController{
		logger: logger,
		db:     db,
		store:  management.NewState(db),
		engine: engine,
	}
}

type AdminsController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *management.State
	engine *rbac.Engine
}

func (srv *AdminsController) GetProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	adminID, err := adminActorID(actor)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.String("admin_id", actor.ID))
	logger.Info("getting profile")

	if err := srv.engine.Allowed(ctx, rbac.Read, rbac.OrganizationScope(actor.OrganizationID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	admin, err := srv.store.GetAdmin(ctx, adminID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("admin not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("admin not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("profile retrieved")
	json.Write(w, http.StatusOK, admin.OAPI())
}

func (srv *AdminsController) Whoami(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	adminID, err := adminActorID(actor)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	if err := srv.engine.Allowed(ctx, rbac.Read, rbac.OrganizationScope(actor.OrganizationID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.String("admin_id", actor.ID))
	logger.Info("getting current admin")

	admin, err := srv.store.GetAdmin(ctx, adminID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("admin not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("admin not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("current admin retrieved")
	json.Write(w, http.StatusOK, admin.OAPI())
}

func (srv *AdminsController) ListAdmins(w http.ResponseWriter, r *http.Request, params oapi.ListAdminsParams) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.OrganizationScope(actor.OrganizationID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("organization_id", actor.OrganizationID.String()),
	)

	pagination := store.Pagination{
		Limit:  20,
		Offset: 0,
	}

	if params.Limit != nil {
		pagination.Limit = params.Limit.ToInt()
	}
	if params.Offset != nil {
		pagination.Offset = params.Offset.ToInt()
	}

	search := params.Search.ToString()

	logger.Info("listing admins", zap.Int("limit", pagination.Limit), zap.Int("offset", pagination.Offset))

	admins, total, err := srv.store.ListAdmins(ctx, actor.OrganizationID, pagination, search)
	if err != nil {
		logger.Error("failed to list admins", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	results := make([]oapi.Admin, len(admins))
	for i, a := range admins {
		results[i] = a.OAPI()
	}

	logger.Info("admins listed", zap.Int("total", total), zap.Int("count", len(results)))

	response := oapi.AdminList{
		Results: results,
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
	}

	json.Write(w, http.StatusOK, response)
}

func (srv *AdminsController) CreateAdmin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	err := srv.engine.Allowed(ctx, rbac.Create, rbac.OrganizationScope(actor.OrganizationID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var body oapi.CreateAdmin
	if err := json.Decode(r.Body, &body); err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(
		zap.String("organization_id", actor.OrganizationID.String()),
		zap.String("email", string(body.Email)),
		zap.String("role", string(body.Role)),
	)

	logger.Info("creating or updating admin")

	existingAdmin, err := srv.store.GetAdminByEmail(ctx, string(body.Email))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		logger.Error("failed to check existing admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if existingAdmin != nil {
		logger = logger.With(zap.String("admin_id", existingAdmin.ID.String()))
		logger.Info("adding existing admin to organization")

		// The email already belongs to a registered admin — possibly one whose
		// home organization is a DIFFERENT org. We must NOT overwrite that admin's
		// GLOBAL identity (email/name/role): the caller only has authority over
		// their own organization, and rewriting another org's admin record would
		// be a cross-org privilege escalation. Adding an existing person to an org
		// is purely an organization-scoped membership grant; their global record
		// is left untouched. The requested role becomes their membership role in
		// THIS organization only.
		_, err := access.ProvisionMembership(ctx, srv.db, srv.engine,
			func(ctx context.Context, _ *management.State) (uuid.UUID, access.Membership, error) {
				return existingAdmin.ID, access.Membership{
					OrganizationID: actor.OrganizationID,
					Role:           string(body.Role),
				}, nil
			},
		)
		if err != nil {
			logger.Error("failed to provision organization membership", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to add admin to organization")))
			return
		}

		// Return the admin's current (unchanged) record.
		admin, err := srv.store.GetAdmin(ctx, existingAdmin.ID)
		if err != nil {
			logger.Error("failed to get admin", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}

		logger.Info("existing admin added to organization")
		json.Write(w, http.StatusCreated, admin.OAPI())
		return
	}

	// Create the admin record and its organization membership atomically: the
	// admin row is inserted inside the same transaction as the membership upsert,
	// and the RBAC tuples are written only after the transaction commits. This
	// prevents partial state (an admin with no membership, or a membership with
	// no role tuples) on a mid-sequence failure.
	adminID, err := access.ProvisionMembership(ctx, srv.db, srv.engine,
		func(ctx context.Context, tx *management.State) (uuid.UUID, access.Membership, error) {
			adminID, err := tx.CreateAdmin(ctx, management.Admin{
				OrganizationID: actor.OrganizationID,
				Email:          string(body.Email),
				FirstName:      body.FirstName,
				LastName:       body.LastName,
				Role:           string(body.Role),
			})
			if err != nil {
				return uuid.Nil, access.Membership{}, err
			}
			return adminID, access.Membership{
				OrganizationID: actor.OrganizationID,
				Role:           string(body.Role),
			}, nil
		},
	)
	if err != nil {
		logger.Error("failed to create admin", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to create admin")))
		return
	}

	createdAdmin, err := srv.store.GetAdmin(ctx, adminID)
	if err != nil {
		logger.Error("failed to get created admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("admin created", zap.String("admin_id", adminID.String()))
	json.Write(w, http.StatusCreated, createdAdmin.OAPI())
}

// ListMyOrganizations returns the organizations the authenticated admin belongs
// to, flagging the one that currently scopes their session.
func (srv *AdminsController) ListMyOrganizations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	adminID, err := adminActorID(actor)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	orgs, err := srv.store.ListOrganizationsForAdmin(ctx, adminID)
	if err != nil {
		srv.logger.Error("failed to list organizations for admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	results := make([]oapi.AdminOrganization, len(orgs))
	for i, o := range orgs {
		results[i] = oapi.AdminOrganization{
			Id:   o.ID,
			Name: o.Name,
			Role: o.Role,
			// IsActive is derived from the RESOLVED active organization
			// (actor.OrganizationID), not from the raw active_organization_id
			// column. This is intentional: resolveActiveOrganization may have
			// fallen back (e.g. the stored active org was revoked), so the actor's
			// org is the org that actually scopes this request — which is exactly
			// what the switcher should show as active. Do not "fix" this to read
			// the stored column.
			IsActive: o.ID == actor.OrganizationID,
		}
	}

	json.Write(w, http.StatusOK, struct {
		Results []oapi.AdminOrganization `json:"results"`
	}{Results: results})
}

// SetActiveOrganization switches the authenticated admin's active organization,
// which scopes subsequent requests. The admin must be a member of the target.
func (srv *AdminsController) SetActiveOrganization(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	adminID, err := adminActorID(actor)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var body oapi.SetActiveOrganizationJSONRequestBody
	if err := json.Decode(r.Body, &body); err != nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	// IDOR guard: an admin may only activate an organization they are a current
	// member of. The membership check is the authorization boundary here, so a DB
	// error must surface as a clean 500 — never as a 403 (which would let a
	// transient failure masquerade as "not a member") and never leaking the raw
	// error to the client.
	isMember, err := srv.store.IsMember(ctx, body.OrganizationId, adminID)
	if err != nil {
		srv.logger.Error("failed to check organization membership", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}
	if !isMember {
		oapi.WriteProblem(w, problem.ErrForbidden(problem.Describe("you are not a member of this organization")))
		return
	}

	if err := srv.store.SetActiveOrganization(ctx, adminID, body.OrganizationId); err != nil {
		srv.logger.Error("failed to set active organization", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (srv *AdminsController) GetAdmin(w http.ResponseWriter, r *http.Request, adminID uuid.UUID) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.OrganizationScope(actor.OrganizationID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("organization_id", actor.OrganizationID.String()),
		zap.String("admin_id", adminID.String()),
	)

	logger.Info("getting admin")

	admin, err := srv.store.GetAdmin(ctx, adminID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("admin not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("admin not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	inOrg, err := srv.store.IsMember(ctx, actor.OrganizationID, admin.ID)
	if err != nil {
		logger.Error("failed to check organization membership", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	if !inOrg {
		logger.Info("admin not in organization")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("admin not found")))
		return
	}

	logger.Info("admin retrieved")
	json.Write(w, http.StatusOK, admin.OAPI())
}

func (srv *AdminsController) UpdateAdmin(w http.ResponseWriter, r *http.Request, adminID uuid.UUID) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.OrganizationScope(actor.OrganizationID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	admin, err := srv.store.GetAdmin(ctx, adminID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("admin not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("admin not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	inOrg, err := srv.store.IsMember(ctx, actor.OrganizationID, admin.ID)
	if err != nil {
		srv.logger.Error("failed to check organization membership", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	if !inOrg {
		srv.logger.Info("admin not in organization")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("admin not found")))
		return
	}

	var body oapi.UpdateAdmin
	if err := json.Decode(r.Body, &body); err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(
		zap.String("organization_id", actor.OrganizationID.String()),
		zap.String("admin_id", adminID.String()),
	)

	logger.Info("updating admin")

	var email *string
	var role *string

	if body.Email != nil {
		emailStr := string(*body.Email)
		email = &emailStr
	}

	if body.Role != nil {
		roleStr := string(*body.Role)
		role = &roleStr
	}

	update := management.AdminUpdate{
		Email:     email,
		FirstName: body.FirstName,
		LastName:  body.LastName,
		Role:      role,
	}

	err = srv.store.UpdateAdmin(ctx, adminID, update)
	if err != nil {
		logger.Error("failed to update admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	updatedAdmin, err := srv.store.GetAdmin(ctx, adminID)
	if err != nil {
		logger.Error("failed to get updated admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("admin updated")
	json.Write(w, http.StatusOK, updatedAdmin.OAPI())
}

func (srv *AdminsController) DeleteAdmin(w http.ResponseWriter, r *http.Request, adminID uuid.UUID) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.OrganizationScope(actor.OrganizationID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	member, err := srv.store.GetMember(ctx, actor.OrganizationID, adminID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("admin not in organization")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("admin not found")))
		return
	}
	if err != nil {
		srv.logger.Error("failed to check organization membership", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("organization_id", actor.OrganizationID.String()),
		zap.String("admin_id", adminID.String()),
	)

	logger.Info("removing admin from organization")

	// Read the grants to revoke before anything is torn down; project access
	// resolves from the direct project:<id>#<role> tuple alone, so dropping the
	// organization membership without these leaves a removed person with full,
	// working access to every project they were ever added to.
	projectRoles, err := srv.store.ListProjectRolesInOrganization(ctx, actor.OrganizationID, adminID)
	if err != nil {
		logger.Error("failed to list project roles for admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Revocation inverts the provisioning order: tuples come out BEFORE the rows
	// that justified them, including before the transaction that removes those
	// rows. The provisioning path (see [access.ProvisionMembership]) is the mirror
	// image and writes tuples only after its commit.
	//
	// The asymmetry is deliberate, and it is the reason this cannot simply run
	// after the commit like a grant does. OpenFGA is not part of the Postgres
	// transaction, so one of the two orders has to lose:
	//
	//   - tuples last: a tuple failure leaves the membership rows deleted and the
	//     access live, with no row left for a replay to find. The endpoint answers
	//     404 on retry and the access is stranded forever — fail-open, permanent.
	//   - tuples first: a rollback (or a failure before the commit) leaves the
	//     person without access they are still recorded as having — fail-closed,
	//     visible, and repaired either by replaying the removal or by re-adding
	//     them, which re-provisions the tuples.
	//
	// The second failure is the one worth having. Losing access you should still
	// hold is an outage; keeping access you should have lost is a breach.
	grants := make([]access.ProjectRoleGrant, len(projectRoles))
	for i, pr := range projectRoles {
		grants[i] = access.ProjectRoleGrant{UserID: adminID, ProjectID: pr.ProjectID, Role: pr.Role}
	}
	if err := access.DeprovisionProjectRoles(ctx, srv.engine, grants); err != nil {
		logger.Error("failed to revoke project RBAC tuples", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to revoke project roles")))
		return
	}

	if err := access.DeprovisionOrganizationRole(ctx, srv.engine, adminID, actor.OrganizationID, member.Role); err != nil {
		logger.Error("failed to revoke organization RBAC tuple", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to revoke organization role")))
		return
	}

	// Membership is the unit of removal now that an admin can belong to several
	// organizations; the admin record is preserved so their other memberships
	// keep working. The soft-delete and the home/active-org reconciliation run in
	// one transaction so an admin can never be left pointing at an org they no
	// longer belong to.
	tx, err := srv.db.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	defer tx.Rollback() //nolint:errcheck

	txStore := management.NewState(tx)

	if err := txStore.RemoveProjectAdminsInOrganization(ctx, actor.OrganizationID, adminID); err != nil {
		logger.Error("failed to remove project memberships", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if err := txStore.RemoveMember(ctx, actor.OrganizationID, adminID); err != nil {
		logger.Error("failed to remove organization membership", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Clear a now-dangling active_organization_id and re-point the home org if it
	// was the removed org, so correctness does not rely solely on the read-time
	// fallback in resolveActiveOrganization.
	if err := txStore.ReconcileAdminOrganizations(ctx, actor.OrganizationID, adminID); err != nil {
		logger.Error("failed to reconcile admin organizations", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if err := tx.Commit(); err != nil {
		logger.Error("failed to commit transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("admin removed from organization")
	w.WriteHeader(http.StatusNoContent)
}

// membersResource is the RBAC resource type that governs the project_admins
// roster: who may see, add, re-role and remove the members of a project.
const membersResource = "members"

func (srv *AdminsController) ListProjectAdmins(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListProjectAdminsParams) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope(membersResource, projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
	)

	pagination := store.Pagination{
		Limit:  20,
		Offset: 0,
	}

	if params.Limit != nil {
		pagination.Limit = params.Limit.ToInt()
	}
	if params.Offset != nil {
		pagination.Offset = params.Offset.ToInt()
	}

	search := params.Search.ToString()

	logger.Info("listing project admins", zap.Int("limit", pagination.Limit), zap.Int("offset", pagination.Offset))

	projectAdmins, total, err := srv.store.ListProjectAdmins(ctx, projectID, pagination, search)
	if err != nil {
		logger.Error("failed to list project admins", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("project admins listed", zap.Int("total", total), zap.Int("count", len(projectAdmins)))

	response := oapi.ProjectAdminList{
		Results: management.ProjectAdmins(projectAdmins).OAPI(),
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
	}

	json.Write(w, http.StatusOK, response)
}

func (srv *AdminsController) GetProjectAdmin(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, adminID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope(membersResource, projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("admin_id", adminID.String()),
	)

	logger.Info("getting project admin")

	projectAdmin, err := srv.store.GetProjectAdmin(ctx, projectID, adminID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("project admin not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("project admin not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get project admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("project admin retrieved")
	json.Write(w, http.StatusOK, projectAdmin.OAPI())
}

func (srv *AdminsController) UpdateProjectAdmin(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, adminID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope(membersResource, projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var body oapi.UpdateProjectAdmin
	if err := json.Decode(r.Body, &body); err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("admin_id", adminID.String()),
		zap.String("role", string(body.Role)),
	)

	logger.Info("updating project admin role")

	newRole := string(body.Role)

	// Fail closed on a role the hierarchy cannot rank. The OpenAPI enum already
	// constrains body.Role, so this is defense in depth: an unranked role would
	// rank 0 and slip under the least-privilege ceiling below, and it would be
	// written to OpenFGA as a relation the model does not define.
	if !rbac.IsKnownProjectRole(newRole) {
		logger.Debug("unknown project role")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("unknown project role")))
		return
	}

	// Least-privilege ceiling, identical to the one the invite flow applies: a
	// caller may only assign a role at or below their own effective role in this
	// project. The two paths grant the same thing and must not disagree, or the
	// weaker one becomes the way around the stronger.
	actorRole, err := srv.actorProjectRole(ctx, projectID)
	if err != nil {
		logger.Error("failed to resolve actor project role", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}
	if rbac.IsProjectRoleHigher(newRole, actorRole) {
		logger.Debug("assigned role is higher than actor project role", zap.String("actor_project_role", actorRole))
		oapi.WriteProblem(w, problem.ErrForbidden(problem.Describe("the role you assign must be equal to or lower than your own role in this project")))
		return
	}

	projectAdmin, err := srv.store.GetProjectAdmin(ctx, projectID, adminID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("project admin not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("project admin not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get project admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if err := srv.guardLastProjectAdmin(ctx, projectID, adminID, projectAdmin.Role, newRole); err != nil {
		logger.Info("project admin role change refused", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// A role change is a revoke and a grant at once, so it obeys both halves of
	// the ordering rule: the old role's tuple goes BEFORE the row is rewritten,
	// the new role's tuple AFTER. Never the other way round — updating the row
	// first would make the retry unrepairable, because the retry reads its
	// "before" value from the very field the failed attempt overwrote and would
	// compute a no-op swap, stranding a demoted member's admin tuple forever.
	//
	// Failing between the steps leaves the member holding no role while the row
	// still names the old one: fail-closed, and a replay re-reads the untouched
	// row and redoes both halves, each of which is idempotent.
	if projectAdmin.Role != newRole {
		if err := access.DeprovisionProjectRole(ctx, srv.engine, adminID, projectID, projectAdmin.Role); err != nil {
			logger.Error("failed to revoke previous RBAC project role", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to update project role")))
			return
		}
	}

	err = srv.store.UpdateProjectAdminRole(ctx, projectID, adminID, newRole)
	if err != nil {
		logger.Error("failed to update project admin role", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if err := access.ProvisionProjectRole(ctx, srv.engine, adminID, projectID, newRole); err != nil {
		logger.Error("failed to write RBAC project role", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to update project role")))
		return
	}

	updatedProjectAdmin, err := srv.store.GetProjectAdmin(ctx, projectID, adminID)
	if err != nil {
		logger.Error("failed to get updated project admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("project admin role updated")
	json.Write(w, http.StatusOK, updatedProjectAdmin.OAPI())
}

func (srv *AdminsController) DeleteProjectAdmin(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, adminID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope(membersResource, projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("admin_id", adminID.String()),
	)

	logger.Info("deleting project admin")

	projectAdmin, err := srv.store.GetProjectAdmin(ctx, projectID, adminID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("project admin not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("project admin not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get project admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if err := srv.guardLastProjectAdmin(ctx, projectID, adminID, projectAdmin.Role, ""); err != nil {
		logger.Info("project admin removal refused", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// The tuple goes first, the row second. Deleting the row first and failing
	// on the tuple would be unrecoverable: the access would remain while the
	// only record pointing at it is gone, and a replay would answer 404 instead
	// of finishing the job. This way a failure leaves access already revoked and
	// a stale roster entry that a replay reads and cleans up.
	if err := access.DeprovisionProjectRole(ctx, srv.engine, adminID, projectID, projectAdmin.Role); err != nil {
		logger.Error("failed to revoke RBAC tuples for project admin", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to revoke project role")))
		return
	}

	err = srv.store.DeleteProjectAdmin(ctx, projectID, adminID)
	if err != nil {
		logger.Error("failed to delete project admin", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("project admin deleted")
	w.WriteHeader(http.StatusNoContent)
}

// guardLastProjectAdmin refuses a change that would leave the project with
// nobody able to administer it. currentRole is the member's role today and
// newRole the role they are moving to ("" when they are being removed
// entirely); the guard only engages when project admin is actually being given
// up.
//
// Organization owners and admins count as administrators here even without a
// project_admins row, because they hold project admin by inheritance. Ignoring
// them would turn this guard into a permanent block on removing a departed
// project admin from an organization that is perfectly able to manage it.
func (srv *AdminsController) guardLastProjectAdmin(ctx context.Context, projectID, adminID uuid.UUID, currentRole, newRole string) error {
	if currentRole != rbac.ProjectAdmin || newRole == rbac.ProjectAdmin {
		return nil
	}

	hasOther, err := srv.store.HasOtherProjectAdmin(ctx, projectID, adminID, rbac.ProjectAdmin, rbac.OrganizationRolesInheritingProjectAdmin())
	if err != nil {
		// Fail closed: an unanswered "is anyone else in charge?" must not be
		// read as a yes.
		return err
	}
	if !hasOther {
		return problem.ErrConflict(problem.Describe("this is the last administrator of the project; assign another administrator first"))
	}
	return nil
}

// actorProjectRole returns the effective project role of the request's actor,
// for use as a least-privilege ceiling.
//
// An admin is resolved from the database — their membership role in the active
// organization folded with any explicit project_admins row — because that is the
// resolution the invite flow applies, and the two grant paths must not disagree.
//
// Any other subject (an API key, an access policy) has no organization
// membership to read; its project role lives in OpenFGA as the direct grant
// written when it was provisioned, so it is resolved from there. Refusing such
// an actor instead would break a configuration the model explicitly supports:
// every resource relation is a union of a direct grant and the project-role
// path, so a policy carrying a custom permission set can hold members:update
// without being a project admin. Such a policy holds no project role at all and
// resolves to "", which ranks below every role and lets it assign none — the
// ceiling doing its job rather than a blanket refusal.
func (srv *AdminsController) actorProjectRole(ctx context.Context, projectID uuid.UUID) (string, error) {
	actor := rbac.FromContext(ctx)
	if actor == nil {
		return "", problem.ErrUnauthorized()
	}

	if actor.Type == rbac.ActorAdmin {
		// Authentication always builds an admin actor from admin.ID, so this
		// never fails in practice. It stays an error rather than a fallback: a
		// fallback would resolve to no role and deny silently, which is a far
		// harder thing to diagnose than a failed request.
		adminID, err := uuid.Parse(actor.ID)
		if err != nil {
			return "", err
		}
		return resolveProjectRole(ctx, srv.store, actor.OrganizationID, projectID, adminID)
	}

	return srv.engine.ProjectRole(ctx, actor.UserKey(), projectID)
}
