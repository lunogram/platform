package v1

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/pkg/claim"
	"github.com/lunogram/platform/pkg/http/json"
	"github.com/lunogram/platform/pkg/http/problem"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/lunogram/platform/services/nexus/oapi"
	"go.uber.org/zap"
)

func NewAdminsController(logger *zap.Logger, db *sqlx.DB) *AdminsController {
	return &AdminsController{
		logger: logger,
		db:     db,
		store:  store.NewStores(db),
	}
}

type AdminsController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *store.Stores
}

func (srv *AdminsController) GetProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	logger := srv.logger.With(zap.String("subject", session.Subject))
	logger.Info("getting profile")

	admin, err := srv.store.GetAdminBySubject(ctx, session)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("admin not found")
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
