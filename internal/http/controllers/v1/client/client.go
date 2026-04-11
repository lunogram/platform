package v1

import (
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

func NewClientController(logger *zap.Logger, db, mgmtDB *sqlx.DB, usrs *subjects.State, pub pubsub.Publisher, engine *rbac.Engine) *ClientController {
	return &ClientController{
		logger: logger,
		db:     db,
		mgmt:   management.NewState(mgmtDB),
		users:  usrs,
		pubsub: pub,
		engine: engine,
	}
}

type ClientController struct {
	logger *zap.Logger
	db     *sqlx.DB
	mgmt   *management.State
	users  *subjects.State
	pubsub pubsub.Publisher
	engine *rbac.Engine
}
