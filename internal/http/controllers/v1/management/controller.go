package v1

import (
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/storage"
	"go.uber.org/zap"
)

func NewController(logger *zap.Logger, db *sqlx.DB, cfg config.Node, storage storage.Storage, pub pubsub.Publisher, registry *providers.Registry) (_ *Controller, err error) {
	controller := &Controller{
		ProjectsController:      NewProjectsController(logger, db),
		CampaignsController:     NewCampaignsController(logger, db),
		TemplatesController:     NewTemplatesController(logger, db),
		AdminsController:        NewAdminsController(logger, db),
		UsersController:         NewUsersController(logger, pub, db, cfg.Storage.MaxUploadSize),
		EventsController:        NewEventsController(logger, db),
		TagsController:          NewTagsController(logger, db),
		LocalesController:       NewLocalesController(logger, db),
		JourneysController:      NewJourneysController(logger, db),
		OrganizationsController: NewOrganizationsController(logger, db),
		ListsController:         NewListsController(logger, db, pub, cfg.Storage.MaxUploadSize),
		DocumentsController:     NewDocumentsController(logger, db, storage, cfg.Storage.MaxUploadSize),
		ProvidersController:     NewProvidersController(logger, db, registry),
		SubscriptionsController: NewSubscriptionsController(logger, db),
	}

	controller.AuthController, err = NewAuthController(logger, db, cfg)
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
}
