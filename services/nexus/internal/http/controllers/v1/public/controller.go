package v1

import (
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/services/nexus/internal/pubsub"
	"go.uber.org/zap"
)

func NewController(logger *zap.Logger, db *sqlx.DB, pub pubsub.Publisher) *Controller {
	return &Controller{
		ClientController: NewClientController(logger, db, pub),
	}
}

type Controller struct {
	*ClientController
}
