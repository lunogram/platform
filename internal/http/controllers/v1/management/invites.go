//go:build !enterprise

package v1

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

type InviteController struct {
	logger *zap.Logger
	mgmt   *management.State
	engine *rbac.Engine
	db     *sqlx.DB
	cfg    config.Invites
}

func NewInviteController(logger *zap.Logger, mgmt *management.State, engine *rbac.Engine, db *sqlx.DB, cfg config.Invites) *InviteController {
	return &InviteController{
		logger: logger,
		mgmt:   mgmt,
		engine: engine,
		db:     db,
		cfg:    cfg,
	}
}

func (srv *InviteController) CreateProjectInvite(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("project invites are not available in the open-source version")))
}

func (srv *InviteController) GetInviteDetails(w http.ResponseWriter, r *http.Request, encryptionPair string) {
	oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("project invites are not available in the open-source version")))
}

func (srv *InviteController) AcceptProjectInvite(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, token string) {
	oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("project invites are not available in the open-source version")))
}

func (srv *InviteController) RevokeProjectInvite(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, tokenNouncePair string) {
	oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("project invites are not available in the open-source version")))
}

func (srv *InviteController) ListProjectInvites(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListProjectInvitesParams) {
	oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("project invites are not available in the open-source version")))
}
