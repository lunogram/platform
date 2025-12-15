package v1

import (
	"net/http"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

func NewController(logger *zap.Logger, db *sqlx.DB, platformProxy http.Handler) *Controller {
	return &Controller{
		ClientController: NewClientController(logger, db, platformProxy),
	}
}

type Controller struct {
	*ClientController
}
