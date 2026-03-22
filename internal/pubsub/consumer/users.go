package consumer

import (
	"context"
	"encoding/json"
	"time"

	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// UsersHandler creates a handler that processes incoming users and stores them in the database.
func UsersHandler(logger *zap.Logger, usrs *subjects.State, pub pubsub.Publisher) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		user := schemas.User{}
		err := json.Unmarshal(msg.Data(), &user)
		if err != nil {
			logger.Error("failed to unmarshal user message", zap.Error(err))
			return Permanent(err)
		}

		logger.Info("incoming user", zap.Stringer("user_id", user.ID), zap.Stringer("project_id", user.ProjectID))

		err = PublishUserRecomputeLists(ctx, logger, usrs, pub, user)
		if err != nil {
			logger.Error("failed to publish user recompute lists", zap.Error(err))
			return err
		}

		if user.Data != nil {
			err = pub.Publish(ctx, schemas.UsersSchema(user.ProjectID), user)
			if err != nil {
				logger.Error("failed to publish user to project subject", zap.Error(err))
				return err
			}
		}

		err = PublishUserEvents(ctx, logger, pub, user)
		if err != nil {
			logger.Error("failed to publish user events", zap.Error(err))
			return err
		}

		logger.Info("user processed successfully", zap.Stringer("user_id", user.ID))
		return nil
	}
}

func PublishUserRecomputeLists(ctx context.Context, logger *zap.Logger, usrs *subjects.State, pub pubsub.Publisher, user schemas.User) error {
	result, err := usrs.SelectListUsersDependency(ctx, user.ProjectID)
	if err != nil {
		logger.Error("failed to list rule user dependencies", zap.Error(err))
		return err
	}

	for _, id := range result {
		list := RecomputeList{
			ID:        id,
			ProjectID: user.ProjectID,
		}

		err = pub.Publish(ctx, schemas.ListsRecompute(user.ProjectID, list.ID), list)
		if err != nil {
			logger.Error("failed to publish rule to project subject", zap.Error(err))
			return err
		}
	}

	return nil
}

func PublishUserEvents(ctx context.Context, logger *zap.Logger, pub pubsub.Publisher, user schemas.User) (err error) {
	// NOTE: the user is created, let's publish a different event
	if user.Version == 0 {
		err = pub.Publish(ctx, schemas.UserEventsProcess(user.ProjectID), user.UserEvent(schemas.EventUserCreated))
		if err != nil {
			logger.Error("failed to publish user created event", zap.Error(err))
			return err
		}

		err = PublishUserAnniversarySchedule(ctx, logger, pub, user)
		if err != nil {
			logger.Error("failed to publish user anniversary schedule", zap.Error(err))
			return err
		}

		return nil
	}

	err = pub.Publish(ctx, schemas.UserEventsProcess(user.ProjectID), user.UserEvent(schemas.EventUserUpdated))
	if err != nil {
		logger.Error("failed to publish user updated event", zap.Error(err))
		return err
	}

	return nil
}

// PublishUserAnniversarySchedule publishes a recurring yearly schedule for
// a newly created user, anchored to their creation date.
func PublishUserAnniversarySchedule(ctx context.Context, logger *zap.Logger, pub pubsub.Publisher, user schemas.User) error {
	now := time.Now()
	interval := "1 year"

	msg := schemas.ScheduledMsg{
		ProjectID:   user.ProjectID,
		Name:        ScheduleAnniversary,
		Type:        "recurring",
		SubjectType: string(subjects.SubjectTypeUser),
		StartAt:     &now,
		Interval:    &interval,
		UserID:      user.ID,
		ExternalId:  user.ExternalID,
		AnonymousId: user.AnonymousID,
		Data:        map[string]any{},
	}

	err := pub.Publish(ctx, schemas.ScheduledProcess(user.ProjectID), msg)
	if err != nil {
		logger.Error("failed to publish user anniversary scheduled message", zap.Error(err))
		return err
	}

	return nil
}

// UserSchemasHandler creates a handler that extracts and stores event schema information.
func UserSchemasHandler(logger *zap.Logger, usrs *subjects.State) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		user := schemas.User{}
		err := json.Unmarshal(msg.Data(), &user)
		if err != nil {
			logger.Error("failed to unmarshal user message", zap.Error(err))
			return Permanent(err)
		}

		logger.Info("incoming user schema", zap.Stringer("user_id", user.ID), zap.Stringer("project_id", user.ProjectID))

		paths := rules.ParsePaths(user.Data)
		err = usrs.UpsertUserSchema(ctx, user.ProjectID, paths)
		if err != nil {
			logger.Error("failed to upsert user schema", zap.Error(err))
			return err
		}

		logger.Info("user schema processed successfully", zap.Stringer("user_id", user.ID))
		return nil
	}
}
