package v1

import (
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/actions"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/gallery"
	"github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/consumer"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/storage"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/lunogram/platform/internal/webhook"
	"github.com/nats-io/nats.go/jetstream"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func NewController(logger *zap.Logger, managementDB, usersDB, journeyDB *sqlx.DB, cfg config.Node, storage storage.Storage, urlResolver *storage.URLResolver, pub pubsub.Publisher, req pubsub.Caller, jet jetstream.JetStream, registry *providers.Registry, actionRegistry *actions.Registry, engine *rbac.Engine, rdb *goredis.Client) (_ *Controller, err error) {
	mgmt := management.NewState(managementDB, management.WithRedis(rdb, cfg.Redis.KeyPrefix))
	projects := management.NewProjectsStore(managementDB)
	usrs := subjects.NewState(usersDB, logger)

	// The deprecated single-URL variables are read here, and only here, so that
	// a deployment that has not yet written a WEBHOOK_CONFIG_FILE keeps working.
	// Deleting this block is what removes the compatibility path.
	//nolint:staticcheck // SA1019: reading the deprecated settings is this call's purpose
	hooks, err := webhook.NewEngine(logger.Named("webhook"), cfg.Webhook.ConfigFile, webhook.LegacyEnv{
		ProjectCreatedURL:     cfg.Webhook.ProjectCreatedURL,
		ProjectCreatedTimeout: cfg.Webhook.ProjectCreatedTimeout,
		EmailTemplatesURL:     cfg.Webhook.EmailTemplatesURL,
		EmailTemplatesTimeout: cfg.Webhook.EmailTemplatesTimeout,
		ForwardAuthorization:  cfg.Webhook.LegacyForwardAuthorization,
	})
	if err != nil {
		return nil, err
	}

	templateGallery, err := gallery.New(logger.Named("email-templates"), hooks.Gallery())
	if err != nil {
		return nil, err
	}

	controller := &Controller{
		ProjectsController:         NewProjectsController(logger, managementDB, usersDB, journeyDB, hooks, pub, engine),
		CampaignsController:        NewCampaignsController(logger, managementDB, usersDB, engine),
		TemplatesController:        NewTemplatesController(logger, managementDB, usersDB, pubsub.NewEmailRenderer(req), registry, engine, cfg.Link.SecretBytes(), cfg.Link.TrackingBaseURL()),
		ActionsController:          NewActionsController(logger, managementDB, pubsub.NewActionCaller(req), usersDB, actionRegistry, engine),
		AdminsController:           NewAdminsController(logger, managementDB, engine),
		UsersController:            NewUsersController(logger, pub, usersDB, journeyDB, mgmt, cfg.Storage.MaxUploadSize, engine),
		EventsController:           NewEventsController(logger, usersDB, engine),
		ScheduledController:        NewScheduledController(logger, usrs, pub, engine),
		TagsController:             NewTagsController(logger, managementDB, engine),
		LocalesController:          NewLocalesController(logger, managementDB, engine),
		JourneysController:         NewJourneysController(logger, journeyDB, usersDB, mgmt, pub, jet, engine, consumer.Namespace(cfg.Nats.Namespace)),
		OrganizationsController:    NewOrganizationsController(logger, usersDB, mgmt, pub, engine),
		ListsController:            NewListsController(logger, usersDB, projects, pub, cfg.Storage.MaxUploadSize, engine),
		DocumentsController:        NewDocumentsController(logger, managementDB, storage, cfg.Storage.MaxUploadSize, urlResolver, engine),
		ProvidersController:        NewProvidersController(logger, managementDB, registry, engine, cfg.PublicBaseURL()),
		SubscriptionsController:    NewSubscriptionsController(logger, managementDB, engine),
		AuthMethodsController:      NewAuthMethodsController(logger, managementDB, engine),
		EmailTemplatesController:   NewEmailTemplatesController(logger, templateGallery, engine),
		SenderIdentitiesController: NewSenderIdentitiesController(logger, managementDB, engine),
		PushProvidersController:    NewPushProvidersController(logger, managementDB, registry, engine),
		BroadcastsController:       NewBroadcastsController(logger, managementDB, usersDB, pub, jet, engine, consumer.Namespace(cfg.Nats.Namespace)),
		InviteController:           NewInviteController(logger, mgmt, engine, managementDB),
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
	*ScheduledController
	*TagsController
	*LocalesController
	*JourneysController
	*OrganizationsController
	*ListsController
	*DocumentsController
	*ProvidersController
	*SubscriptionsController
	*AuthController
	*AuthMethodsController
	*ActionsController
	*EmailTemplatesController
	*SenderIdentitiesController
	*PushProvidersController
	*BroadcastsController
	*InviteController
}
