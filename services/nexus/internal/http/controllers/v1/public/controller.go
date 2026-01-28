package v1

import (
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/services/nexus/internal/pubsub"
	"go.uber.org/zap"
)

func NewController(logger *zap.Logger, db *sqlx.DB, pub pubsub.Publisher) (*Controller, error) {
	subsController, err := NewSubscriptionsController(logger, db)
	if err != nil {
		return nil, err
	}

	return &Controller{
		ClientController:        NewClientController(logger, db, pub),
		SubscriptionsController: subsController,
	}, nil
}

type Controller struct {
	*ClientController
	*SubscriptionsController
}
