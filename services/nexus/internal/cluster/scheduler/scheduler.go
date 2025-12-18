package scheduler

import (
	"context"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"go.uber.org/zap"
)

func NewController(ctx graceful.Context, logger *zap.Logger, config config.Node) *Controller {
	return &Controller{
		logger: logger,
		config: config,
	}
}

type Controller struct {
	logger *zap.Logger
	config config.Node
}

func (controller *Controller) Release(ctx context.Context, id string) error {
	return nil
}

func (controller *Controller) Schedule(ctx context.Context) {
	reconciliation := time.NewTicker(controller.config.Cluster.SchedulerInterval)
	defer reconciliation.Stop()

	logger := controller.logger.With(zap.String("component", "scheduler"))
	logger.Info("starting scheduler")

	for {
		select {
		case <-ctx.Done():
			logger.Info("stopping scheduler")
			return
		case <-reconciliation.C:
			logger.Debug("reconciling cluster state")
		}
	}
}
