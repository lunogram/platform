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

// ReconcileUserScheduledEvents queries for due user scheduled events and publishes them
// as UserEvents through the standard event pipeline. For each due event it:
//  1. Publishes a UserEvent to UserEventsProcess with name "scheduled.<schedule_name>"
//  2. Marks the event as fired
//  3. For recurring schedules, advances to the next cycle when the offset_minutes=0 event fires
func (controller *Controller) ReconcileUserScheduledEvents(ctx context.Context) func() {
	return func() {
		defer controller.recover("user_scheduled_events")
		start := time.Now()
		var published, failed int

		scanner := func(event subjects.DueScheduledEvent) error {
			eventName := fmt.Sprintf("scheduled.%s", event.ScheduleName)

			data := make(map[string]any)
			if event.Data != nil {
				err := json.Unmarshal(event.Data, &data)
				if err != nil {
					controller.logger.Warn("failed to unmarshal scheduled event data, using empty payload",
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

			userEvent := schemas.UserEvent{
				ID:        uuid.New(),
				Name:      eventName,
				ProjectID: event.ProjectID,
				UserID:    event.UserID,
				Data:      data,
			}

			// NOTE: There is an intentional gap between Publish and MarkScheduledEventFired.
			// If the process crashes after publishing but before marking fired, the event will
			// be re-published on the next reconciliation cycle. Downstream consumers MUST be
			// idempotent to handle duplicate deliveries safely.
			err := controller.pub.Publish(ctx, schemas.UserEventsProcess(event.ProjectID), userEvent)
			if err != nil {
				failed++
				metrics.ScheduledEventsFireFailuresTotal.WithLabelValues("user").Inc()
				controller.logger.Error("failed to publish scheduled user event",
					zap.Error(err),
					zap.String("event_name", eventName),
					zap.Stringer("user_id", event.UserID),
				)
				return nil
			}

			published++
			metrics.ScheduledEventsFiredTotal.WithLabelValues("user").Inc()
			metrics.ScheduledEventsFireDelaySeconds.WithLabelValues("user").Observe(time.Since(event.FireAt).Seconds())
			controller.logger.Debug("published scheduled user event",
				zap.String("event_name", eventName),
				zap.Stringer("user_id", event.UserID),
				zap.Stringer("project_id", event.ProjectID),
				zap.Stringer("schedule_id", event.ScheduleID),
				zap.String("offset", event.Offset),
				zap.Time("fire_at", event.FireAt),
			)

			err = controller.scheduled.MarkScheduledEventFired(ctx, event.ID)
			if err != nil {
				controller.logger.Error("failed to mark scheduled event as fired",
					zap.Error(err),
					zap.Stringer("event_id", event.ID),
				)
				return nil
			}

			return nil
		}

		processed, err := controller.scheduled.ScanDueScheduledEvents(ctx, scanner)
		if err != nil {
			controller.logger.Error("failed to scan due scheduled events", zap.Error(err))
		}

		controller.logger.Debug("reconciled user scheduled events",
			zap.Int("processed", processed),
			zap.Int("published", published),
			zap.Int("failed", failed),
		)

		metrics.ReconciliationRunsTotal.WithLabelValues("user_scheduled_events").Inc()
		metrics.ReconciliationItemsProcessedTotal.WithLabelValues("user_scheduled_events").Add(float64(processed))
		metrics.ReconciliationItemsPublishedTotal.WithLabelValues("user_scheduled_events").Add(float64(published))
		metrics.ReconciliationItemsFailedTotal.WithLabelValues("user_scheduled_events").Add(float64(failed))
		metrics.ReconciliationDurationSeconds.WithLabelValues("user_scheduled_events").Observe(time.Since(start).Seconds())
	}
}

// ReconcileUserSchedules scans recurring user_schedules whose start_at is in the past
// and that have no pending (unfired) user_scheduled_events. For each one it advances
// start_at by the interval until it is in the future and generates the next batch of
// user_scheduled_events for all offsets.
func (controller *Controller) ReconcileUserSchedules(ctx context.Context) func() {
	return func() {
		defer controller.recover("user_schedules")
		start := time.Now()
		var advanced, failed int

		scanner := func(us subjects.UserSchedule) error {
			err := controller.scheduled.AdvanceAndGenerateUserScheduleEvents(ctx, us)
			if err != nil {
				failed++
				metrics.SchedulesAdvanceFailuresTotal.WithLabelValues("user").Inc()
				controller.logger.Error("failed to advance user schedule",
					zap.Error(err),
					zap.Stringer("user_schedule_id", us.ID),
					zap.Stringer("user_id", us.UserID),
					zap.Stringer("schedule_id", us.ScheduleID),
				)
				return nil
			}

			advanced++
			metrics.SchedulesAdvancedTotal.WithLabelValues("user").Inc()
			controller.logger.Debug("advanced recurring user schedule",
				zap.Stringer("user_schedule_id", us.ID),
				zap.Stringer("user_id", us.UserID),
				zap.Stringer("schedule_id", us.ScheduleID),
			)

			return nil
		}

		processed, err := controller.scheduled.ScanRecurringUserSchedulesWithoutPendingEvents(ctx, scanner)
		if err != nil {
			controller.logger.Error("failed to scan recurring user schedules", zap.Error(err))
		}

		controller.logger.Debug("reconciled user schedules",
			zap.Int("processed", processed),
			zap.Int("advanced", advanced),
			zap.Int("failed", failed),
		)

		metrics.ReconciliationRunsTotal.WithLabelValues("user_schedules").Inc()
		metrics.ReconciliationItemsProcessedTotal.WithLabelValues("user_schedules").Add(float64(processed))
		metrics.ReconciliationItemsPublishedTotal.WithLabelValues("user_schedules").Add(float64(advanced))
		metrics.ReconciliationItemsFailedTotal.WithLabelValues("user_schedules").Add(float64(failed))
		metrics.ReconciliationDurationSeconds.WithLabelValues("user_schedules").Observe(time.Since(start).Seconds())
	}
}
