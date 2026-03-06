package v1

import (
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/actions"
	"github.com/lunogram/platform/internal/config"
	mgmtoapi "github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/storage"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/webhook"
	"github.com/nats-io/nats.go/jetstream"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"
)

func NewController(logger *zap.Logger, managementDB, usersDB, journeyDB *sqlx.DB, cfg config.Node, storage storage.Storage, pub pubsub.Publisher, req pubsub.Caller, jet jetstream.JetStream, registry *providers.Registry, actionRegistry *actions.Registry, engine *rbac.Engine) (_ *Controller, err error) {
	mgmt := management.NewState(managementDB)
	projects := management.NewProjectsStore(managementDB)

	// Create webhook caller for project creation notifications
	webhookCaller := webhook.NewCaller(logger.Named("webhook"), cfg.Webhook)

	controller := &Controller{
		ProjectsController:      NewProjectsController(logger, managementDB, usersDB, journeyDB, webhookCaller, pub, engine),
		CampaignsController:     NewCampaignsController(logger, managementDB, usersDB, engine),
		TemplatesController:     NewTemplatesController(logger, managementDB, engine),
		AdminsController:        NewAdminsController(logger, managementDB, engine),
		UsersController:         NewUsersController(logger, pub, usersDB, journeyDB, mgmt, cfg.Storage.MaxUploadSize, engine),
		EventsController:        NewEventsController(logger, usersDB, engine),
		TagsController:          NewTagsController(logger, managementDB, engine),
		LocalesController:       NewLocalesController(logger, managementDB, engine),
		JourneysController:      NewJourneysController(logger, journeyDB, usersDB, mgmt, pub, jet, engine),
		OrganizationsController: NewOrganizationsController(logger, usersDB, pub, engine),
		ListsController:         NewListsController(logger, usersDB, projects, pub, cfg.Storage.MaxUploadSize, engine),
		DocumentsController:     NewDocumentsController(logger, managementDB, storage, cfg.Storage.MaxUploadSize, engine),
		ProvidersController:     NewProvidersController(logger, managementDB, registry, engine),
		SubscriptionsController: NewSubscriptionsController(logger, managementDB, engine),
		ApiKeysController:       NewApiKeysController(logger, managementDB, engine),
		ActionsController:       NewActionsController(logger, managementDB, req, usersDB, actionRegistry, engine),
	}

	controller.AuthController, err = NewAuthController(logger, managementDB, cfg, engine)
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
	*ActionsController
}

func (c *Controller) ListJourneyEntrances(w http.ResponseWriter, r *http.Request, projectID openapi_types.UUID, journeyID openapi_types.UUID, params mgmtoapi.ListJourneyEntrancesParams) {
	w.WriteHeader(http.StatusNotImplemented)
}
