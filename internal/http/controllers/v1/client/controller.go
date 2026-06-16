package v1

import (
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

func NewController(logger *zap.Logger, mgmtDB, usersDB *sqlx.DB, mgmt *management.State, usrs *subjects.State, pub pubsub.Publisher, engine *rbac.Engine) (*Controller, error) {
	clientController := NewClientController(logger, usersDB, mgmtDB, usrs, pub, engine)

	subsController, err := NewSubscriptionsController(clientController, mgmtDB, mgmt)
	if err != nil {
		return nil, err
	}

	return &Controller{
		UsersController:         NewUsersController(clientController),
		EventsController:        NewEventsController(clientController),
		OrganizationsController: NewOrganizationsController(clientController),
		ScheduledController:     NewScheduledController(clientController),
		InboxController:         NewInboxController(clientController),
		DevicesController:       NewDevicesController(clientController),
		SubscriptionsController: subsController,
	}, nil
}

type Controller struct {
	*UsersController
	*EventsController
	*OrganizationsController
	*ScheduledController
	*InboxController
	*DevicesController
	*SubscriptionsController
}
