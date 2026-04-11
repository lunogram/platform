//go:build !enterprise

package scheduler

import "context"

// ReconcileScheduledBroadcasts is a no-op in OSS builds. Scheduled broadcasts
// require the enterprise build tag.
func (controller *Controller) ReconcileScheduledBroadcasts(_ context.Context) func() {
	return func() {}
}
