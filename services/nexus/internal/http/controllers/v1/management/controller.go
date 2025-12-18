package v1

import (
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/storage"
	"go.uber.org/zap"
)

func NewController(logger *zap.Logger, db *sqlx.DB, cfg config.Node, storage storage.Storage, platformProxy http.Handler) *Controller {
	return &Controller{
		ProjectsController:      NewProjectsController(logger, db),
		CampaignsController:     NewCampaignsController(logger, db),
		TemplatesController:     NewTemplatesController(logger, db),
		AdminsController:        NewAdminsController(logger, db),
		UsersController:         NewUsersController(logger, db),
		TagsController:          NewTagsController(logger, db),
		LocalesController:       NewLocalesController(logger, db),
		JourneysController:      NewJourneysController(logger, db),
		OrganizationsController: NewOrganizationsController(logger, db),
		ListsController:         NewListsController(logger, db, cfg.Storage.MaxUploadSize),
		DocumentsController:     NewDocumentsController(logger, db, storage, cfg.Storage.MaxUploadSize),
	}
}

type Controller struct {
	*ProjectsController
	*CampaignsController
	*TemplatesController
	*AdminsController
	*UsersController
	*TagsController
	*LocalesController
	*JourneysController
	*OrganizationsController
	*ListsController
	*DocumentsController
}
