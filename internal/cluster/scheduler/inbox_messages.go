package scheduler

import (
	"context"
	"time"

	"github.com/lunogram/platform/internal/node/metrics"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

func (controller *Controller) ReconcileUserInboxMessages(ctx context.Context) func() {
	return func() {
		defer controller.recover("user_inbox_messages")
		start := time.Now()
		var published, failed int

		scanner := func(message subjects.InboxMessage) error {
			if message.UserID == nil {
				failed++
				controller.logger.Warn("due user inbox message has no user_id", zap.Stringer("message_id", message.ID))
				return nil
			}

			// Delegate the full lifecycle (inbox.message.created event,
			// provider dispatch, sent_at tracking, inbox.message.sent
			// event) to the inbox handler.
			err := controller.pub.Publish(ctx, schemas.UserInboxProcess(message.ProjectID), schemas.InboxMessage{
				ProjectID: message.ProjectID,
				MessageID: message.ID,
				SubjectID: *message.UserID,
				Channel:   string(message.Channel),
			})
			if err != nil {
				failed++
				controller.logger.Error("failed to publish scheduled user inbox event", zap.Error(err), zap.Stringer("message_id", message.ID))
				return nil
			}

			published++
			controller.logger.Debug("published scheduled user inbox event", zap.Stringer("message_id", message.ID), zap.Stringer("user_id", *message.UserID), zap.Time("scheduled_at", message.ScheduledAt))
			return nil
		}

		processed, err := controller.inbox.ScanDueUserInboxMessages(ctx, controller.reconciliationBatchSize, scanner)
		if err != nil {
			controller.logger.Error("failed to scan due user inbox messages", zap.Error(err))
		}

		controller.logger.Debug("reconciled user inbox messages", zap.Int("processed", processed), zap.Int("published", published), zap.Int("failed", failed))
		metrics.ReconciliationRunsTotal.WithLabelValues("user_inbox_messages").Inc()
		metrics.ReconciliationItemsProcessedTotal.WithLabelValues("user_inbox_messages").Add(float64(processed))
		metrics.ReconciliationItemsPublishedTotal.WithLabelValues("user_inbox_messages").Add(float64(published))
		metrics.ReconciliationItemsFailedTotal.WithLabelValues("user_inbox_messages").Add(float64(failed))
		metrics.ReconciliationDurationSeconds.WithLabelValues("user_inbox_messages").Observe(time.Since(start).Seconds())
	}
}

func (controller *Controller) ReconcileOrganizationInboxMessages(ctx context.Context) func() {
	return func() {
		defer controller.recover("organization_inbox_messages")
		start := time.Now()
		var published, failed int

		scanner := func(message subjects.InboxMessage) error {
			if message.OrganizationID == nil {
				failed++
				controller.logger.Warn("due organization inbox message has no organization_id", zap.Stringer("message_id", message.ID))
				return nil
			}

			// Delegate the full lifecycle (inbox.message.created event,
			// provider dispatch, sent_at tracking, inbox.message.sent
			// event) to the inbox handler.
			err := controller.pub.Publish(ctx, schemas.OrganizationInboxProcess(message.ProjectID), schemas.InboxMessage{
				ProjectID: message.ProjectID,
				MessageID: message.ID,
				SubjectID: *message.OrganizationID,
				Channel:   string(message.Channel),
			})
			if err != nil {
				failed++
				controller.logger.Error("failed to publish scheduled organization inbox event", zap.Error(err), zap.Stringer("message_id", message.ID))
				return nil
			}

			published++
			controller.logger.Debug("published scheduled organization inbox event", zap.Stringer("message_id", message.ID), zap.Stringer("organization_id", *message.OrganizationID), zap.Time("scheduled_at", message.ScheduledAt))
			return nil
		}

		processed, err := controller.inbox.ScanDueOrganizationInboxMessages(ctx, controller.reconciliationBatchSize, scanner)
		if err != nil {
			controller.logger.Error("failed to scan due organization inbox messages", zap.Error(err))
		}

		controller.logger.Debug("reconciled organization inbox messages", zap.Int("processed", processed), zap.Int("published", published), zap.Int("failed", failed))
		metrics.ReconciliationRunsTotal.WithLabelValues("organization_inbox_messages").Inc()
		metrics.ReconciliationItemsProcessedTotal.WithLabelValues("organization_inbox_messages").Add(float64(processed))
		metrics.ReconciliationItemsPublishedTotal.WithLabelValues("organization_inbox_messages").Add(float64(published))
		metrics.ReconciliationItemsFailedTotal.WithLabelValues("organization_inbox_messages").Add(float64(failed))
		metrics.ReconciliationDurationSeconds.WithLabelValues("organization_inbox_messages").Observe(time.Since(start).Seconds())
	}
}
