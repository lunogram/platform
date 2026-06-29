package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/node/metrics"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	iredis "github.com/lunogram/platform/internal/redis"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// resolveUserID ensures scheduled.UserID is populated, looking it up by
// identifiers when necessary.
func resolveUserID(ctx context.Context, logger *zap.Logger, txState *subjects.State, scheduled *schemas.ScheduledMsg) error {
	if scheduled.UserID != uuid.Nil {
		return nil
	}

	if len(scheduled.Identifiers) == 0 {
		return fmt.Errorf("user_id or identifiers are required for user scheduled")
	}

	logger.Info("looking up user ID from identifiers")

	var err error
	scheduled.UserID, err = txState.LookupUserID(ctx, scheduled.ProjectID, scheduled.Identifiers)
	if err != nil {
		logger.Error("failed to lookup user ID", zap.Error(err))
		return err
	}
	return nil
}

// resolveOrganizationID ensures scheduled.OrganizationID is populated, looking
// it up by identifiers when necessary.
func resolveOrganizationID(ctx context.Context, logger *zap.Logger, txState *subjects.State, scheduled *schemas.ScheduledMsg) error {
	if scheduled.OrganizationID != uuid.Nil {
		return nil
	}

	if len(scheduled.Identifiers) == 0 {
		return fmt.Errorf("organization_id or identifiers are required for organization scheduled")
	}

	var err error
	scheduled.OrganizationID, err = txState.LookupOrganizationID(ctx, scheduled.ProjectID, scheduled.Identifiers)
	if err != nil {
		logger.Error("failed to lookup organization ID", zap.Error(err))
		return err
	}
	return nil
}

// ScheduledHandler creates a handler that processes incoming scheduled entities,
// upserts the schedule definition, upserts user/org schedule assignments, and
// fans out to schema processing.
//
// All database mutations (UpsertSchedule + UpsertUserSchedule or
// UpsertOrganizationSchedule) are wrapped in a single transaction so they
// either all succeed or all roll back.
func ScheduledHandler(logger *zap.Logger, db *sqlx.DB, usrs *subjects.State, pub pubsub.Publisher, schemaCache *iredis.SchemaCache) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		var scheduled schemas.ScheduledMsg
		if err := json.Unmarshal(msg.Data(), &scheduled); err != nil {
			logger.Error("failed to unmarshal scheduled message", zap.Error(err))
			return Permanent(err)
		}

		logger := logger.With(
			zap.String("name", scheduled.Name),
			zap.String("subject_type", scheduled.SubjectType),
			zap.Stringer("project_id", scheduled.ProjectID),
		)
		logger.Info("incoming scheduled")

		scheduleType := scheduled.Type
		if scheduleType == "" {
			scheduleType = "single"
		}

		data, err := json.Marshal(scheduled.Data)
		if err != nil {
			logger.Error("failed to marshal scheduled data", zap.Error(err))
			return err
		}

		// Convert zero-value ScheduledAt to nil so it doesn't override
		// StartAt in recurring schedule event generation.
		var scheduledAt *time.Time
		if !scheduled.ScheduledAt.IsZero() {
			scheduledAt = &scheduled.ScheduledAt
		}

		// Wrap all mutations in a transaction so the schedule upsert and
		// the subject schedule upsert are atomic.
		tx, err := db.BeginTxx(ctx, nil)
		if err != nil {
			logger.Error("failed to begin transaction", zap.Error(err))
			return err
		}
		defer tx.Rollback() //nolint:errcheck

		txState := subjects.NewState(tx, logger)

		scheduled.ScheduledID, err = txState.UpsertSchedule(ctx, scheduled.ProjectID, scheduled.Name, scheduleType)
		if err != nil {
			logger.Error("failed to upsert schedule", zap.Error(err))
			return err
		}

		switch subjects.SubjectType(scheduled.SubjectType) {
		case subjects.SubjectTypeUser:
			if err := resolveUserID(ctx, logger, txState, &scheduled); err != nil {
				return err
			}
			if _, err := txState.UpsertUserSchedule(ctx, scheduled.AssignmentID, scheduled.UserID, scheduled.ScheduledID, scheduledAt, scheduled.StartAt, scheduled.Interval, data); err != nil {
				if errors.Is(err, subjects.ErrScheduleOwnershipMismatch) {
					logger.Error("user schedule assignment id does not match subject/schedule", zap.Error(err))
					return Permanent(err)
				}
				logger.Error("failed to upsert user schedule", zap.Error(err))
				return err
			}

		case subjects.SubjectTypeOrganization:
			if err := resolveOrganizationID(ctx, logger, txState, &scheduled); err != nil {
				return err
			}
			if _, err := txState.UpsertOrganizationSchedule(ctx, scheduled.AssignmentID, scheduled.OrganizationID, scheduled.ScheduledID, scheduledAt, scheduled.StartAt, scheduled.Interval, data); err != nil {
				if errors.Is(err, subjects.ErrScheduleOwnershipMismatch) {
					logger.Error("organization schedule assignment id does not match subject/schedule", zap.Error(err))
					return Permanent(err)
				}
				logger.Error("failed to upsert organization schedule", zap.Error(err))
				return err
			}

		default:
			logger.Error("unknown subject type")
			return fmt.Errorf("unknown subject_type: %s", scheduled.SubjectType)
		}

		if err := tx.Commit(); err != nil {
			logger.Error("failed to commit transaction", zap.Error(err))
			return err
		}

		metrics.ScheduledEventsIngestedTotal.WithLabelValues(scheduled.SubjectType, scheduleType).Inc()

		if err := PublishScheduledSchema(ctx, logger, pub, scheduled, schemaCache); err != nil {
			logger.Error("failed to publish scheduled schema", zap.Error(err))
			return err
		}

		logger.Info("scheduled processed successfully",
			zap.Stringer("scheduled_id", scheduled.ScheduledID))
		return nil
	}
}

// PublishScheduledSchema publishes the scheduled schema to the schema
// subject if the scheduled entity contains data properties and the data
// shape has not been seen before.
func PublishScheduledSchema(ctx context.Context, logger *zap.Logger, pub pubsub.Publisher, scheduled schemas.ScheduledMsg, schemaCache *iredis.SchemaCache) error {
	if scheduled.Data != nil {
		if schemaCache.Seen(ctx, iredis.Scheduler, scheduled.ScheduledID, scheduled.Data) {
			return nil
		}

		err := pub.Publish(ctx, schemas.ScheduledSchema(scheduled.ProjectID), scheduled)
		if err != nil {
			logger.Error("failed to publish scheduled to schema subject", zap.Error(err))
			return err
		}
	}
	return nil
}

// ScheduledSchemasHandler creates a handler that extracts and stores scheduled schema information
// for both user and organization scheduled entities.
func ScheduledSchemasHandler(logger *zap.Logger, usrs *subjects.State) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		scheduled := schemas.ScheduledMsg{}
		err := json.Unmarshal(msg.Data(), &scheduled)
		if err != nil {
			logger.Error("failed to unmarshal scheduled message", zap.Error(err))
			return Permanent(err)
		}

		logger.Info("incoming scheduled schema",
			zap.Stringer("scheduled_id", scheduled.ScheduledID),
			zap.String("subject_type", scheduled.SubjectType),
			zap.Stringer("project_id", scheduled.ProjectID),
		)

		paths := rules.ParsePaths(scheduled.Data)
		err = usrs.UpsertScheduledSchema(ctx, scheduled.ProjectID, scheduled.ScheduledID, paths)
		if err != nil {
			logger.Error("failed to upsert scheduled schema", zap.Error(err))
			return err
		}

		logger.Info("scheduled schema processed successfully", zap.Stringer("scheduled_id", scheduled.ScheduledID))
		return nil
	}
}

// ScheduledBackfillHandler creates a handler that backfills user_scheduled_events
// and organization_scheduled_events for all existing schedule assignments when a
// new offset is created on a schedule definition.
//
// The backfill is performed entirely in SQL via INSERT...SELECT so that fire_at
// computation (scheduled_at ± offset based on direction) and the future-only
// filter happen in a single round-trip per subject type.
func ScheduledBackfillHandler(logger *zap.Logger, usrs *subjects.State) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		var backfill schemas.ScheduleOffsetBackfillMsg
		if err := json.Unmarshal(msg.Data(), &backfill); err != nil {
			logger.Error("failed to unmarshal schedule offset backfill message", zap.Error(err))
			return Permanent(err)
		}

		logger := logger.With(
			zap.Stringer("project_id", backfill.ProjectID),
			zap.Stringer("schedule_id", backfill.ScheduleID),
			zap.Stringer("offset_id", backfill.OffsetID),
			zap.String("offset", backfill.Offset),
		)

		logger.Info("backfilling scheduled events for new offset")

		// Backfill user scheduled events.
		userCount, err := usrs.BackfillUserScheduledEventsForOffset(ctx, backfill.ScheduleID, backfill.OffsetID)
		if err != nil {
			logger.Error("failed to backfill user scheduled events", zap.Error(err))
			return err
		}

		// Backfill organization scheduled events.
		orgCount, err := usrs.BackfillOrgScheduledEventsForOffset(ctx, backfill.ScheduleID, backfill.OffsetID)
		if err != nil {
			logger.Error("failed to backfill org scheduled events", zap.Error(err))
			return err
		}

		logger.Info("backfill completed for new offset",
			zap.Int64("user_events_created", userCount),
			zap.Int64("org_events_created", orgCount),
		)
		return nil
	}
}
