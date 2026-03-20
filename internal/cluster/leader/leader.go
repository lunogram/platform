package leader

import (
	"context"

	"github.com/lunogram/platform/internal/cluster"
	"github.com/lunogram/platform/internal/cluster/scheduler"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

func NewHandler(scheduler *scheduler.Controller, managementStore *management.State, logger *zap.Logger) cluster.LeaderHandler {
	return func(ctx context.Context) error {
		logger.Info("Trying to create VAPID keys if they don't exist")
		err := managementStore.CreateVapidKeysIfNotExist()
		if err != nil {
			logger.Error("Failed to create VAPID keys", zap.Error(err))
			return err
		}

		go scheduler.Schedule(ctx)
		<-ctx.Done()
		return nil
	}
}
