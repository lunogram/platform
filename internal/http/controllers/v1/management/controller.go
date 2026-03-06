package v1

import (
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/actions"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/storage"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/webhook"
	"go.uber.org/zap"
)

func NewController(logger *zap.Logger, managementDB, usersDB, journeyDB *sqlx.DB, cfg config.Node, storage storage.Storage, pub pubsub.Publisher, req pubsub.Caller, registry *providers.Registry, actionRegistry *actions.Registry) (_ *Controller, err error) {
	mgmt := management.NewState(managementDB)
	projects := management.NewProjectsStore(managementDB)

	// Create webhook caller for project creation notifications
	webhookCaller := webhook.NewCaller(logger.Named("webhook"), cfg.Webhook)

	controller := &Controller{
		ProjectsController:             NewProjectsController(logger, managementDB, usersDB, journeyDB, webhookCaller, pub),
		CampaignsController:            NewCampaignsController(logger, managementDB, usersDB),
		TemplatesController:            NewTemplatesController(logger, managementDB),
		AdminsController:               NewAdminsController(logger, managementDB),
		UsersController:                NewUsersController(logger, pub, usersDB, journeyDB, mgmt, cfg.Storage.MaxUploadSize),
		EventsController:               NewEventsController(logger, usersDB),
		TagsController:                 NewTagsController(logger, managementDB),
		LocalesController:              NewLocalesController(logger, managementDB),
		JourneysController:             NewJourneysController(logger, journeyDB, mgmt),
		OrganizationsController:        NewOrganizationsController(logger, managementDB),
		SubjectOrganizationsController: NewSubjectOrganizationsController(logger, usersDB, pub),
		ListsController:                NewListsController(logger, usersDB, projects, pub, cfg.Storage.MaxUploadSize),
		DocumentsController:            NewDocumentsController(logger, managementDB, storage, cfg.Storage.MaxUploadSize),
		ProvidersController:            NewProvidersController(logger, managementDB, registry),
		SubscriptionsController:        NewSubscriptionsController(logger, managementDB),
		ApiKeysController:              NewApiKeysController(logger, managementDB),
		ActionsController:              NewActionsController(logger, managementDB, req, usersDB, actionRegistry),
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
	*SubjectOrganizationsController
	*ListsController
	*DocumentsController
	*ProvidersController
	*SubscriptionsController
	*AuthController
	*ApiKeysController
	*ActionsController
}
