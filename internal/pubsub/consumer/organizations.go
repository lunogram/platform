package consumer

import (
	"context"
	"encoding/json"
	"time"

	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	iredis "github.com/lunogram/platform/internal/redis"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// OrganizationsHandler creates a handler that processes incoming organizations and stores them in the database.
func OrganizationsHandler(logger *zap.Logger, usrs *subjects.State, pub pubsub.Publisher, schemaCache *iredis.SchemaCache) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		org := schemas.Organization{}
		err := json.Unmarshal(msg.Data(), &org)
		if err != nil {
			logger.Error("failed to unmarshal organization message", zap.Error(err))
			return Permanent(err)
		}

		logger.Info("incoming organization", zap.Stringer("organization_id", org.ID), zap.Stringer("project_id", org.ProjectID))

		err = PublishOrganizationRecomputeLists(ctx, logger, usrs, pub, org)
		if err != nil {
			logger.Error("failed to publish organization recompute lists", zap.Error(err))
			return err
		}

		if org.Data != nil {
			if !schemaCache.Seen(ctx, iredis.Organization, org.ProjectID, org.Data) {
				err = pub.Publish(ctx, schemas.OrganizationsSchema(org.ProjectID), org)
				if err != nil {
					logger.Error("failed to publish organization to schema subject", zap.Error(err))
					return err
				}
			}
		}

		err = PublishOrganizationEvents(ctx, logger, pub, org)
		if err != nil {
			logger.Error("failed to publish organization events", zap.Error(err))
			return err
		}

		logger.Info("organization processed successfully", zap.Stringer("organization_id", org.ID))
		return nil
	}
}

// PublishOrganizationRecomputeLists publishes recompute messages for all lists that depend on organization data.
func PublishOrganizationRecomputeLists(ctx context.Context, logger *zap.Logger, usrs *subjects.State, pub pubsub.Publisher, org schemas.Organization) error {
	result, err := usrs.SelectListOrganizationsDependency(ctx, org.ProjectID)
	if err != nil {
		logger.Error("failed to list rule organization dependencies", zap.Error(err))
		return err
	}

	for _, id := range result {
		list := RecomputeList{
			ID:        id,
			ProjectID: org.ProjectID,
		}

		err = pub.Publish(ctx, schemas.ListsRecompute(org.ProjectID, list.ID), list)
		if err != nil {
			logger.Error("failed to publish list recompute", zap.Error(err))
			return err
		}
	}

	return nil
}

// PublishOrganizationEvents publishes organization system events (organization.created or organization.updated).
func PublishOrganizationEvents(ctx context.Context, logger *zap.Logger, pub pubsub.Publisher, org schemas.Organization) error {
	if org.Version == 0 {
		err := pub.Publish(ctx, schemas.OrganizationEventsProcess(org.ProjectID), org.OrganizationEvent(schemas.EventOrganizationCreated))
		if err != nil {
			logger.Error("failed to publish organization created event", zap.Error(err))
			return err
		}

		err = PublishOrganizationAnniversarySchedule(ctx, logger, pub, org)
		if err != nil {
			logger.Error("failed to publish organization anniversary schedule", zap.Error(err))
			return err
		}

		return nil
	}

	err := pub.Publish(ctx, schemas.OrganizationEventsProcess(org.ProjectID), org.OrganizationEvent(schemas.EventOrganizationUpdated))
	if err != nil {
		logger.Error("failed to publish organization updated event", zap.Error(err))
		return err
	}

	return nil
}

// PublishOrganizationAnniversarySchedule publishes a recurring yearly schedule for
// a newly created organization, anchored to their creation date.
func PublishOrganizationAnniversarySchedule(ctx context.Context, logger *zap.Logger, pub pubsub.Publisher, org schemas.Organization) error {
	now := time.Now()
	interval := "1 year"

	msg := schemas.ScheduledMsg{
		ProjectID:      org.ProjectID,
		Name:           ScheduleAnniversary,
		Type:           "recurring",
		SubjectType:    string(subjects.SubjectTypeOrganization),
		StartAt:        &now,
		Interval:       &interval,
		OrganizationID: org.ID,
		Identifiers:    org.Identifiers,
		Data:           map[string]any{},
	}

	err := pub.Publish(ctx, schemas.ScheduledProcess(org.ProjectID), msg)
	if err != nil {
		logger.Error("failed to publish organization anniversary scheduled message", zap.Error(err))
		return err
	}

	return nil
}

// OrganizationSchemasHandler creates a handler that extracts and stores organization schema information.
func OrganizationSchemasHandler(logger *zap.Logger, usrs *subjects.State) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		org := schemas.Organization{}
		err := json.Unmarshal(msg.Data(), &org)
		if err != nil {
			logger.Error("failed to unmarshal organization message", zap.Error(err))
			return Permanent(err)
		}

		logger.Info("incoming organization schema", zap.Stringer("organization_id", org.ID), zap.Stringer("project_id", org.ProjectID))

		paths := rules.ParsePaths(org.Data)
		err = usrs.UpsertOrganizationSchema(ctx, org.ProjectID, paths)
		if err != nil {
			logger.Error("failed to upsert organization schema", zap.Error(err))
			return err
		}

		logger.Info("organization schema processed successfully", zap.Stringer("organization_id", org.ID))
		return nil
	}
}

// OrganizationUsersHandler creates a handler that processes incoming organization user memberships.
func OrganizationUsersHandler(logger *zap.Logger, usrs *subjects.State, pub pubsub.Publisher, schemaCache *iredis.SchemaCache) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		orgUser := schemas.OrganizationUser{}
		err := json.Unmarshal(msg.Data(), &orgUser)
		if err != nil {
			logger.Error("failed to unmarshal organization user message", zap.Error(err))
			return Permanent(err)
		}

		logger.Info("incoming organization user", zap.Stringer("organization_id", orgUser.OrganizationID), zap.Stringer("user_id", orgUser.UserID), zap.Stringer("project_id", orgUser.ProjectID))

		err = PublishOrganizationUserRecomputeLists(ctx, logger, usrs, pub, orgUser)
		if err != nil {
			logger.Error("failed to publish organization user recompute lists", zap.Error(err))
			return err
		}

		if orgUser.Data != nil {
			if !schemaCache.Seen(ctx, iredis.OrganizationMembers, orgUser.ProjectID, orgUser.Data) {
				err = pub.Publish(ctx, schemas.OrganizationUsersSchema(orgUser.ProjectID), orgUser)
				if err != nil {
					logger.Error("failed to publish organization user to schema subject", zap.Error(err))
					return err
				}
			}
		}

		err = PublishOrganizationUserEvents(ctx, logger, pub, orgUser)
		if err != nil {
			logger.Error("failed to publish organization user events", zap.Error(err))
			return err
		}

		logger.Info("organization user processed successfully", zap.Stringer("organization_id", orgUser.OrganizationID), zap.Stringer("user_id", orgUser.UserID))
		return nil
	}
}

// PublishOrganizationUserRecomputeLists publishes recompute messages for all lists that depend on organization user data.
func PublishOrganizationUserRecomputeLists(ctx context.Context, logger *zap.Logger, usrs *subjects.State, pub pubsub.Publisher, orgUser schemas.OrganizationUser) error {
	result, err := usrs.SelectListOrganizationUsersDependency(ctx, orgUser.ProjectID)
	if err != nil {
		logger.Error("failed to list rule organization user dependencies", zap.Error(err))
		return err
	}

	for _, id := range result {
		list := RecomputeList{
			ID:        id,
			ProjectID: orgUser.ProjectID,
		}

		err = pub.Publish(ctx, schemas.ListsRecompute(orgUser.ProjectID, list.ID), list)
		if err != nil {
			logger.Error("failed to publish list recompute", zap.Error(err))
			return err
		}
	}

	return nil
}

// PublishOrganizationUserEvents publishes organization user system events (organization.user.added or organization.user.updated).
func PublishOrganizationUserEvents(ctx context.Context, logger *zap.Logger, pub pubsub.Publisher, orgUser schemas.OrganizationUser) error {
	if orgUser.Version == 0 {
		err := pub.Publish(ctx, schemas.OrganizationEventsProcess(orgUser.ProjectID), orgUser.OrganizationEvent(schemas.EventOrganizationUserAdded))
		if err != nil {
			logger.Error("failed to publish organization user added event", zap.Error(err))
			return err
		}

		return nil
	}

	err := pub.Publish(ctx, schemas.OrganizationEventsProcess(orgUser.ProjectID), orgUser.OrganizationEvent(schemas.EventOrganizationUserUpdated))
	if err != nil {
		logger.Error("failed to publish organization user updated event", zap.Error(err))
		return err
	}

	return nil
}

// OrganizationUserSchemasHandler creates a handler that extracts and stores organization user schema information.
func OrganizationUserSchemasHandler(logger *zap.Logger, usrs *subjects.State) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		orgUser := schemas.OrganizationUser{}
		err := json.Unmarshal(msg.Data(), &orgUser)
		if err != nil {
			logger.Error("failed to unmarshal organization user message", zap.Error(err))
			return Permanent(err)
		}

		logger.Info("incoming organization user schema", zap.Stringer("organization_id", orgUser.OrganizationID), zap.Stringer("user_id", orgUser.UserID), zap.Stringer("project_id", orgUser.ProjectID))

		paths := rules.ParsePaths(orgUser.Data)
		err = usrs.UpsertOrganizationUserSchema(ctx, orgUser.ProjectID, paths)
		if err != nil {
			logger.Error("failed to upsert organization user schema", zap.Error(err))
			return err
		}

		logger.Info("organization user schema processed successfully", zap.Stringer("organization_id", orgUser.OrganizationID), zap.Stringer("user_id", orgUser.UserID))
		return nil
	}
}
