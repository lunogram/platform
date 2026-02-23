package v1

import (
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/storage"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

func NewController(logger *zap.Logger, managementDB, usersDB, journeyDB *sqlx.DB, cfg config.Node, storage storage.Storage, jet jetstream.JetStream, registry *providers.Registry) (_ *Controller, err error) {
	mgmt := management.NewState(managementDB)
	projects := management.NewProjectsStore(managementDB)

	pub := pubsub.NewPublisher(jet)
	controller := &Controller{
		ProjectsController:      NewProjectsController(logger, managementDB, usersDB, journeyDB),
		CampaignsController:     NewCampaignsController(logger, managementDB, usersDB),
		TemplatesController:     NewTemplatesController(logger, managementDB),
		AdminsController:        NewAdminsController(logger, managementDB),
		UsersController:         NewUsersController(logger, pub, usersDB, journeyDB, mgmt, cfg.Storage.MaxUploadSize),
		EventsController:        NewEventsController(logger, usersDB),
		TagsController:          NewTagsController(logger, managementDB),
		LocalesController:       NewLocalesController(logger, managementDB),
		JourneysController:      NewJourneysController(logger, journeyDB, mgmt, jet, pub),
		OrganizationsController: NewOrganizationsController(logger, managementDB),
		ListsController:         NewListsController(logger, usersDB, projects, pub, cfg.Storage.MaxUploadSize),
		DocumentsController:     NewDocumentsController(logger, managementDB, storage, cfg.Storage.MaxUploadSize),
		ProvidersController:     NewProvidersController(logger, managementDB, registry),
		SubscriptionsController: NewSubscriptionsController(logger, managementDB),
		ApiKeysController:       NewApiKeysController(logger, managementDB),
	}

	controller.AuthController, err = NewAuthController(logger, managementDB, cfg)
	if err != nil {
		return nil, err
	}

	return controller, nil
}

type Controller struct {
	*ProjectsController
	*CampaignsController
	*TemplatesController
	*AdminsController
	*UsersController
	*EventsController
	*TagsController
	*LocalesController
	*JourneysController
	*OrganizationsController
	*ListsController
	*DocumentsController
	*ProvidersController
	*SubscriptionsController
	*AuthController
	*ApiKeysController
}
