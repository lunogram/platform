package v1

import (
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

func NewController(logger *zap.Logger, mgmtDB, usersDB *sqlx.DB, mgmt *management.State, usrs *subjects.State, pub pubsub.Publisher) (*Controller, error) {
	subsController, err := NewSubscriptionsController(logger, mgmtDB, mgmt, usrs)
	if err != nil {
		return nil, err
	}

	return &Controller{
		ClientController:        NewClientController(logger, usersDB, usrs, pub),
		SubscriptionsController: subsController,
	}, nil
}

type Controller struct {
	*ClientController
	*SubscriptionsController
}
