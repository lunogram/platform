package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/node/metrics"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

// ReconcileOrganizationScheduledEvents queries for due organization scheduled events and publishes them
// as OrganizationEvents through the standard event pipeline. For each due event it:
//  1. Publishes an OrganizationEvent to OrganizationEventsProcess with name "scheduled.<schedule_name>"
//  2. Marks the event as fired
//  3. For recurring schedules, advances to the next cycle when the offset_minutes=0 event fires
func (controller *Controller) ReconcileOrganizationScheduledEvents(ctx context.Context) func() {
	return func() {
		defer controller.recover("organization_scheduled_events")
		start := time.Now()
		var published, failed int

		scanner := func(event subjects.DueOrgScheduledEvent) error {
			eventName := fmt.Sprintf("scheduled.%s", event.ScheduleName)

			data := make(map[string]any)
			if event.Data != nil {
				if err := json.Unmarshal(event.Data, &data); err != nil {
					controller.logger.Warn("failed to unmarshal scheduled org event data, using empty payload",
						zap.Error(err),
						zap.Stringer("event_id", event.ID),
					)
				}
				if data == nil {
					data = make(map[string]any)
				}
			}

			data["schedule_offset_id"] = event.ScheduleOffsetID.String()
			data["offset"] = event.Offset
			data["fire_at"] = event.FireAt.Format(time.RFC3339)
			data["schedule_id"] = event.ScheduleID.String()

			orgEvent := schemas.OrganizationEvent{
				ID:             uuid.New(),
				Name:           eventName,
				ProjectID:      event.ProjectID,
				OrganizationID: event.OrganizationID,
				Data:           data,
			}

			// NOTE: There is an intentional gap between Publish and MarkOrgScheduledEventFired.
			// If the process crashes after publishing but before marking fired, the event will
			// be re-published on the next reconciliation cycle. Downstream consumers MUST be
			// idempotent to handle duplicate deliveries safely.
			err := controller.pub.Publish(ctx, schemas.OrganizationEventsProcess(event.ProjectID), orgEvent)
			if err != nil {
				failed++
				metrics.ScheduledEventsFireFailuresTotal.WithLabelValues("organization").Inc()
				controller.logger.Error("failed to publish scheduled org event",
					zap.Error(err),
					zap.String("event_name", eventName),
					zap.Stringer("organization_id", event.OrganizationID))
				return nil
			}

			published++
			metrics.ScheduledEventsFiredTotal.WithLabelValues("organization").Inc()
			metrics.ScheduledEventsFireDelaySeconds.WithLabelValues("organization").Observe(time.Since(event.FireAt).Seconds())
			controller.logger.Debug("published scheduled org event",
				zap.String("event_name", eventName),
				zap.Stringer("organization_id", event.OrganizationID),
				zap.Stringer("project_id", event.ProjectID),
				zap.Stringer("schedule_id", event.ScheduleID),
				zap.String("offset", event.Offset),
				zap.Time("fire_at", event.FireAt))

			err = controller.scheduled.MarkOrgScheduledEventFired(ctx, event.ID)
			if err != nil {
				controller.logger.Error("failed to mark org scheduled event as fired",
					zap.Error(err),
					zap.Stringer("event_id", event.ID))
				return nil
			}

			return nil
		}

		processed, err := controller.scheduled.ScanDueOrgScheduledEvents(ctx, controller.reconciliationBatchSize, scanner)
		if err != nil {
			controller.logger.Error("failed to scan due org scheduled events", zap.Error(err))
		}

		controller.logger.Debug("reconciled org scheduled events",
			zap.Int("processed", processed),
			zap.Int("published", published),
			zap.Int("failed", failed),
		)

		metrics.ReconciliationRunsTotal.WithLabelValues("organization_scheduled_events").Inc()
		metrics.ReconciliationItemsProcessedTotal.WithLabelValues("organization_scheduled_events").Add(float64(processed))
		metrics.ReconciliationItemsPublishedTotal.WithLabelValues("organization_scheduled_events").Add(float64(published))
		metrics.ReconciliationItemsFailedTotal.WithLabelValues("organization_scheduled_events").Add(float64(failed))
		metrics.ReconciliationDurationSeconds.WithLabelValues("organization_scheduled_events").Observe(time.Since(start).Seconds())
	}
}

// ReconcileOrganizationSchedules scans recurring organization_schedules whose start_at is
// in the past and that have no pending (unfired) organization_scheduled_events. For each
// one it advances start_at by the interval until it is in the future and generates the
// next batch of organization_scheduled_events for all offsets.
func (controller *Controller) ReconcileOrganizationSchedules(ctx context.Context) func() {
	return func() {
		defer controller.recover("organization_schedules")
		start := time.Now()
		var advanced, failed int

		scanner := func(os subjects.OrganizationSchedule) error {
			err := controller.scheduled.AdvanceAndGenerateOrgScheduleEvents(ctx, os)
			if err != nil {
				failed++
				metrics.SchedulesAdvanceFailuresTotal.WithLabelValues("organization").Inc()
				controller.logger.Error("failed to advance organization schedule",
					zap.Error(err),
					zap.Stringer("organization_schedule_id", os.ID),
					zap.Stringer("organization_id", os.OrganizationID),
					zap.Stringer("schedule_id", os.ScheduleID))
				return nil
			}

			advanced++
			metrics.SchedulesAdvancedTotal.WithLabelValues("organization").Inc()
			controller.logger.Debug("advanced recurring organization schedule",
				zap.Stringer("organization_schedule_id", os.ID),
				zap.Stringer("organization_id", os.OrganizationID),
				zap.Stringer("schedule_id", os.ScheduleID))

			return nil
		}

		processed, err := controller.scheduled.ScanRecurringOrgSchedulesWithoutPendingEvents(ctx, controller.reconciliationBatchSize, scanner)
		if err != nil {
			controller.logger.Error("failed to scan recurring organization schedules", zap.Error(err))
		}

		controller.logger.Debug("reconciled organization schedules",
			zap.Int("processed", processed),
			zap.Int("advanced", advanced),
			zap.Int("failed", failed),
		)

		metrics.ReconciliationRunsTotal.WithLabelValues("organization_schedules").Inc()
		metrics.ReconciliationItemsProcessedTotal.WithLabelValues("organization_schedules").Add(float64(processed))
		metrics.ReconciliationItemsPublishedTotal.WithLabelValues("organization_schedules").Add(float64(advanced))
		metrics.ReconciliationItemsFailedTotal.WithLabelValues("organization_schedules").Add(float64(failed))
		metrics.ReconciliationDurationSeconds.WithLabelValues("organization_schedules").Observe(time.Since(start).Seconds())
	}
}
