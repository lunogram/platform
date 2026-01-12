package leader

import (
	"context"

	"github.com/lunogram/platform/services/nexus/internal/cluster"
	"github.com/lunogram/platform/services/nexus/internal/cluster/scheduler"
)

func NewHandler(scheduler *scheduler.Controller) cluster.LeaderHandler {
	return func(ctx context.Context) error {
		go scheduler.Schedule(ctx)
		<-ctx.Done()
		return nil
	}
}
