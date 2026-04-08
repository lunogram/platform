package v1

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

func NewDevicesController(logger *zap.Logger, managementDB, usersDB *sqlx.DB, engine *rbac.Engine) *DevicesController {
	return &DevicesController{
		logger:       logger,
		managementDB: managementDB,
		usersDB:      usersDB,
		engine:       engine,
	}
}

type DevicesController struct {
	logger       *zap.Logger
	managementDB *sqlx.DB
	usersDB      *sqlx.DB
	engine       *rbac.Engine
}

func (srv *DevicesController) GetVapidPublicKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if actor == nil {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	srv.logger.Info("retrieving VAPID public key")

	vapidStore := management.NewVapidKeysStore(srv.managementDB)
	key, err := vapidStore.GetVapidKeyByName("default")
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			srv.logger.Warn("no VAPID key found, creating one")
			err = vapidStore.CreateVapidKeysIfNotExist()
			if err != nil {
				srv.logger.Error("failed to create VAPID keys", zap.Error(err))
				oapi.WriteProblem(w, problem.ErrInternal())
				return
			}
			key, err = vapidStore.GetVapidKeyByName("default")
			if err != nil {
				srv.logger.Error("failed to get VAPID key after creation", zap.Error(err))
				oapi.WriteProblem(w, problem.ErrInternal())
				return
			}
		} else {
			srv.logger.Error("failed to get VAPID key", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}
	}

	response := struct {
		PublicKey string `json:"public_key"`
	}{
		PublicKey: key.PublicKey,
	}

	srv.logger.Info("VAPID public key retrieved successfully")
	json.Write(w, http.StatusOK, response)
}
