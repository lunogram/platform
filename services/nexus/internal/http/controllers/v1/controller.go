package v1

import (
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

func NewController(logger *zap.Logger, db *sqlx.DB) *Controller {
	return &Controller{
		CampaignsController: NewCampaignsController(logger, db),
		TemplatesController: NewTemplatesController(logger, db),
	}
}

type Controller struct {
	*CampaignsController
	*TemplatesController
}
