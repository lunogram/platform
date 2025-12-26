package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/services/nexus/internal/rules"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

type User struct {
	ID          uuid.UUID      `json:"id"`
	ProjectID   uuid.UUID      `json:"project_id"`
	AnonymousID *string        `json:"anonymous_id"`
	ExternalID  *string        `json:"external_id"`
	Email       *string        `json:"email"`
	Phone       *string        `json:"phone"`
	Timezone    *string        `json:"timezone"`
	Locale      *string        `json:"locale"`
	Data        map[string]any `json:"data"`
	Version     int32          `json:"version"`
}

func (u User) Event(name string) Event {
	return Event{
		Name:        name,
		ProjectID:   u.ProjectID,
		AnonymousId: u.AnonymousID,
		ExternalId:  u.ExternalID,
		Data: map[string]any{
			"id":       u.ID,
			"email":    u.Email,
			"phone":    u.Phone,
			"timezone": u.Timezone,
			"locale":   u.Locale,
			"data":     u.Data,
			"version":  u.Version,
		},
	}
}

// UsersProjectSubject returns the NATS subject for project-specific users.
func UsersProjectSubject(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("users.projects.%s", projectID))
}

// UsersSchemaSubject returns the NATS subject for user schema updates.
func UsersSchemaSubject(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("users.schemas.%s", projectID))
}

// UsersHandler creates a handler that processes incoming users and stores them in the database.
func UsersHandler(logger *zap.Logger, db *sqlx.DB, pub Publisher) HandlerFunc {
	lists := store.NewListsStore(db)

	return func(ctx context.Context, msg jetstream.Msg) error {
		user := User{}
		err := json.Unmarshal(msg.Data(), &user)
		if err != nil {
			logger.Error("failed to unmarshal user message", zap.Error(err))
			return err
		}

		logger.Info("incoming user", zap.Stringer("user_id", user.ID), zap.Stringer("project_id", user.ProjectID))

		err = PublishUserRecomputeLists(ctx, logger, lists, pub, user)
		if err != nil {
			logger.Error("failed to publish user recompute lists", zap.Error(err))
			return err
		}

		if user.Data != nil {
			err = pub.Publish(ctx, UsersSchemaSubject(user.ProjectID), user)
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

func PublishUserRecomputeLists(ctx context.Context, logger *zap.Logger, lists *store.ListsStore, pub Publisher, user User) error {
	result, err := lists.SelectListUsersDependency(ctx, user.ProjectID)
	if err != nil {
		logger.Error("failed to list rule user dependencies", zap.Error(err))
		return err
	}

	for _, id := range result {
		list := RecomputeList{
			ID:        id,
			ProjectID: user.ProjectID,
		}

		err = pub.Publish(ctx, RecomputeListSubject(user.ProjectID, list.ID), list)
		if err != nil {
			logger.Error("failed to publish rule to project subject", zap.Error(err))
			return err
		}
	}

	return nil
}

func PublishUserEvents(ctx context.Context, logger *zap.Logger, pub Publisher, user User) (err error) {
	// NOTE: the user is created, let's publish a different event
	if user.Version == 0 {
		err = pub.Publish(ctx, EventsProjectSubject(user.ProjectID), user.Event(EventUserCreated))
		if err != nil {
			logger.Error("failed to publish user created event", zap.Error(err))
			return err
		}

		return nil
	}

	err = pub.Publish(ctx, EventsProjectSubject(user.ProjectID), user.Event(EventUserUpdated))
	if err != nil {
		logger.Error("failed to publish user updated event", zap.Error(err))
		return err
	}

	return nil
}

// UserSchemasHandler creates a handler that extracts and stores event schema information.
func UserSchemasHandler(logger *zap.Logger, db *sqlx.DB) HandlerFunc {
	users := store.NewUsersStore(db)

	return func(ctx context.Context, msg jetstream.Msg) error {
		user := User{}
		err := json.Unmarshal(msg.Data(), &user)
		if err != nil {
			logger.Error("failed to unmarshal user message", zap.Error(err))
			return err
		}

		logger.Info("incoming user schema", zap.Stringer("user_id", user.ID), zap.Stringer("project_id", user.ProjectID))

		paths := rules.ParsePaths(user.Data)
		err = users.UpsertUserSchema(ctx, user.ProjectID, paths)
		if err != nil {
			logger.Error("failed to upsert user schema", zap.Error(err))
			return err
		}

		logger.Info("user schema processed successfully", zap.Stringer("user_id", user.ID))
		return nil
	}
}
