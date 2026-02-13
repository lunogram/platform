package v1

import (
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/users"
	"go.uber.org/zap"
)

func NewController(logger *zap.Logger, db *store.Connections, mgmt *management.State, usrs *users.State, pub pubsub.Publisher) (*Controller, error) {
	subsController, err := NewSubscriptionsController(logger, db.Users, mgmt, usrs)
	if err != nil {
		return nil, err
	}

	return &Controller{
		ClientController:        NewClientController(logger, db.Users, usrs, pub),
		SubscriptionsController: subsController,
	}, nil
}

type Controller struct {
	*ClientController
	*SubscriptionsController
}
