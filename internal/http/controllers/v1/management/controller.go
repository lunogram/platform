package v1

import (
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/storage"
	"github.com/lunogram/platform/internal/store"
	"go.uber.org/zap"
)

func NewController(logger *zap.Logger, db *store.Connections, cfg config.Node, storage storage.Storage, pub pubsub.Publisher, registry *providers.Registry) (_ *Controller, err error) {
	controller := &Controller{
		ProjectsController:      NewProjectsController(logger, db),
		CampaignsController:     NewCampaignsController(logger, db.Management),
		TemplatesController:     NewTemplatesController(logger, db.Management),
		AdminsController:        NewAdminsController(logger, db.Management),
		UsersController:         NewUsersController(logger, pub, db, cfg.Storage.MaxUploadSize),
		EventsController:        NewEventsController(logger, db.Users),
		TagsController:          NewTagsController(logger, db.Management),
		LocalesController:       NewLocalesController(logger, db.Management),
		JourneysController:      NewJourneysController(logger, db.Journey),
		OrganizationsController: NewOrganizationsController(logger, db.Management),
		ListsController:         NewListsController(logger, db, pub, cfg.Storage.MaxUploadSize),
		DocumentsController:     NewDocumentsController(logger, db.Management, storage, cfg.Storage.MaxUploadSize),
		ProvidersController:     NewProvidersController(logger, db.Management, registry),
		SubscriptionsController: NewSubscriptionsController(logger, db.Management),
		ApiKeysController:       NewApiKeysController(logger, db.Management),
	}

	controller.AuthController, err = NewAuthController(logger, db.Management, cfg)
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
