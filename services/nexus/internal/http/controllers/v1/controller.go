package v1

import (
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

func NewController(logger *zap.Logger, db *sqlx.DB) *Controller {
	return &Controller{
		CampaignsController: NewCampaignsController(logger, db),
		TemplatesController: NewTemplatesController(logger, db),
		AdminsController:    NewAdminsController(logger, db),
		UsersController:     NewUsersController(logger, db),
		TagsController:      NewTagsController(logger, db),
		LocalesController:   NewLocalesController(logger, db),
	}
}

type Controller struct {
	*CampaignsController
	*TemplatesController
	*AdminsController
	*UsersController
	*TagsController
	*LocalesController
}
