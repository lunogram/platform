package v1

import (
	"encoding/base64"
	"math/rand"
	"net/http"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/rbac"
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
