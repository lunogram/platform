package v1

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"math/rand"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

type InviteController struct {
	logger *zap.Logger
	mgmt   *management.State
	engine *rbac.Engine
}

func NewInviteController(logger *zap.Logger, mgmt *management.State, engine *rbac.Engine) *InviteController {
	return &InviteController{
		logger: logger,
		mgmt:   mgmt,
		engine: engine,
	}
}

func randomString(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)[:n]
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

	logger := srv.logger.With(zap.String("project_id", projectID.String()), zap.String("email", string(body.Email)))
	logger.Info("creating project invite")

	actor := rbac.FromContext(ctx)
	InviterAdminID := actor.ID

	// at 1 billion invite tokens (~200GB of data), the probability of a collision
	// is 10^-102%, a decimal point followed by 101 zeroes and a 1. if every atom
	// in the observable universe (2^266) was an invite token, it'd still only be
	// ~10^-21%. At that point we're also storing 2^266 * 200 bytes of data, which is a number so
	// large it doesn't have a name. if this ever fires, forget the bug, go buy a lottery ticket.
	token := randomString(50)

	expiresIn := "24h"
	if body.ExpiresIn != nil {
		expiresIn = *body.ExpiresIn
	}

	invite, err := srv.mgmt.CreateProjectInvite(ctx, projectID, InviterAdminID, string(body.Email), body.Role, token, expiresIn)
	if err != nil {
		logger.Error("failed to create project invite", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("Created project invite", zap.String("invite_id", invite.ID.String()))

	response := invite.OAPI()
	json.Write(w, http.StatusOK, response)
}

func (srv *InviteController) GetInviteDetails(w http.ResponseWriter, r *http.Request, token string) {
	ctx := r.Context()

	invite, err := srv.mgmt.GetInviteByToken(ctx, token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			srv.logger.Debug("invite not found", zap.String("token", token))
			oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("invite not found")))
		} else {
			srv.logger.Error("failed to get invite details", zap.String("token", token), zap.Error(err))
			oapi.WriteProblem(w, err)
		}
		return
	}

	response := invite.OAPI()
	json.Write(w, http.StatusOK, response)
}

func (srv *InviteController) AcceptProjectInvite(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, token string) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)

	invite, err := srv.mgmt.GetInviteByToken(ctx, token)
	if err != nil {
		srv.logger.Debug("invite not found", zap.String("token", token), zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	adminId, err := uuid.Parse(actor.ID)
	if err != nil {
		srv.logger.Error("invalid admin ID in token", zap.String("admin_id", actor.ID), zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	err = srv.mgmt.AddAdminToProject(ctx, projectID, adminId, invite.Role)
	if err != nil {
		srv.logger.Error("failed to add admin to project", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	//TODO: When the more more relation of the organizations x Admins is implemented, we should also add the user to the organization that the project belongs to.

	invite, err = srv.mgmt.AcceptProjectInvite(ctx, token)
	if err != nil {
		err = srv.mgmt.HardDeleteProjectAdmin(ctx, projectID, adminId)
		if err != nil {
			srv.logger.Error("failed to rollback project admin addition after invite acceptance failure", zap.String("project_id", projectID.String()), zap.String("admin_id", adminId.String()), zap.Error(err))
		}

		srv.logger.Error("failed to accept project invite", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	response := invite.OAPI()
	json.Write(w, http.StatusOK, response)
}

func (srv *InviteController) RevokeProjectInvite(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, token string) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("invites", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	invite, err := srv.mgmt.RevokeProjectInvite(ctx, token)
	if err != nil {
		srv.logger.Debug("invite not found or already revoked/accepted", zap.String("token", token), zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	response := invite.OAPI()
	json.Write(w, http.StatusOK, response)
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

	invites, total, err := srv.mgmt.ListProjectInvites(ctx, projectID, pagination, params.Search.ToString(), params.Role, params.Status, expiresBefore, expiresAfter)
	if err != nil {
		logger.Error("failed to list project invites", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("listed project invites", zap.Int("count", len(invites)))
	response := make([]oapi.ProjectInvite, len(invites))
	for i, invite := range invites {
		response[i] = invite.OAPI()
	}

	json.Write(w, http.StatusOK, oapi.ProjectInviteListResponse{
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		Results: response,
	})
}
