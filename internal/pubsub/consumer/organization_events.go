package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/node/metrics"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/rules/eval"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// OrganizationEventsHandler creates a handler that processes incoming organization events and stores them in the database.
// It also triggers list recomputation and journey advancement for all users in the organization.
func OrganizationEventsHandler(logger *zap.Logger, usrs *subjects.State, jrny *journey.State, pub pubsub.Publisher) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		start := time.Now()
		event := schemas.OrganizationEvent{}
		err := json.Unmarshal(msg.Data(), &event)
		if err != nil {
			logger.Error("failed to unmarshal organization event message", zap.Error(err))
			metrics.EventsProcessingErrorsTotal.WithLabelValues("organization").Inc()
			return Permanent(err)
		}

		logger.Info("incoming organization event", zap.String("name", event.Name), zap.Stringer("project_id", event.ProjectID))

		event.ID, err = usrs.UpsertEvent(ctx, event.ProjectID, event.Name, subjects.SubjectTypeOrganization)
		if err != nil {
			logger.Error("failed to upsert event", zap.Error(err))
			return err
		}

		if event.OrganizationID == uuid.Nil {
			if len(event.OrganizationIdentifiers) == 0 {
				logger.Error("organization_id or identifiers are required")
				return Permanent(fmt.Errorf("organization_id or identifiers are required for organization event"))
			}

			logger.Info("looking up organization ID from identifiers")

			event.OrganizationID, err = usrs.LookupOrganizationID(ctx, event.ProjectID, event.OrganizationIdentifiers)
			if err != nil {
				logger.Error("failed to lookup organization ID", zap.Error(err))
				return err
			}
		}

		_, err = usrs.InsertOrganizationEvent(ctx, event.OrganizationID, event.ID, event.Data)
		if err != nil {
			logger.Error("failed to insert organization event", zap.Error(err))
			return err
		}

		wg, ctx := errgroup.WithContext(ctx)
		wg.Go(PublishOrganizationEventSchema(ctx, logger, pub, event))
		wg.Go(PublishOrganizationEventListDependencies(ctx, logger, usrs, pub, event))
		wg.Go(PublishOrganizationEventJourneyDependencies(ctx, logger, usrs, jrny, pub, event))

		err = wg.Wait()
		if err != nil {
			logger.Error("failed to publish dependent events", zap.Error(err))
			return err
		}

		logger.Info("organization event processed successfully", zap.Stringer("event_id", event.ID), zap.Stringer("organization_id", event.OrganizationID))
		metrics.EventsProcessedTotal.WithLabelValues("organization").Inc()
		metrics.EventsProcessingDurationSeconds.WithLabelValues("organization").Observe(time.Since(start).Seconds())
		return nil
	}
}

// PublishOrganizationEventSchema returns a function that publishes the organization event schema
// if the event contains data properties.
func PublishOrganizationEventSchema(ctx context.Context, logger *zap.Logger, pub pubsub.Publisher, event schemas.OrganizationEvent) func() error {
	return func() error {
		if event.Data != nil {
			err := pub.Publish(ctx, schemas.OrganizationEventsSchema(event.ProjectID), event)
			if err != nil {
				logger.Error("failed to publish organization event to schema subject", zap.Error(err))
				return err
			}
		}
		return nil
	}
}

// PublishOrganizationEventListDependencies returns a function that publishes recompute messages for all lists
// that depend on the given event through rule conditions.
func PublishOrganizationEventListDependencies(ctx context.Context, logger *zap.Logger, usrs *subjects.State, pub pubsub.Publisher, event schemas.OrganizationEvent) func() error {
	return func() error {
		lists, err := usrs.ListEventListDependencies(ctx, event.ID)
		if err != nil {
			logger.Error("failed to list rule event dependencies", zap.Error(err))
			return err
		}

		for _, id := range lists {
			list := RecomputeList{
				ID:        id,
				ProjectID: event.ProjectID,
			}

			err = pub.Publish(ctx, schemas.ListsRecompute(event.ProjectID, list.ID), list)
			if err != nil {
				logger.Error("failed to publish list recompute", zap.Error(err))
				return err
			}

			metrics.EventsListRecomputesTotal.Inc()
		}

		return nil
	}
}

// PublishOrganizationEventJourneyDependencies returns a function that triggers journey entrance steps
// for users in the organization when an organization event matches a journey entrance condition.
// It evaluates entrance.Rule against event data in-memory, and uses entrance.UserRule to filter
// users in the database. Users are streamed to avoid loading all IDs into memory.
func PublishOrganizationEventJourneyDependencies(ctx context.Context, logger *zap.Logger, usrs *subjects.State, jrny *journey.State, pub pubsub.Publisher, event schemas.OrganizationEvent) func() error {
	evaluator := eval.NewEvaluator()

	return func() error {
		deps, err := jrny.ListEventJourneyDependencies(ctx, event.ID)
		if err != nil {
			logger.Error("failed to list event journey dependencies", zap.Error(err))
			return err
		}

		if len(deps) == 0 {
			return nil
		}

		for _, dep := range deps {
			entrance := oapi.EntranceStepData{}
			if dep.Data != nil {
				err := json.Unmarshal(*dep.Data, &entrance)
				if err != nil {
					return err
				}
			}

			// Evaluate event conditions in-memory
			if entrance.Rule != nil {
				match, err := evaluator.Evaluate(*entrance.Rule, event.Data)
				if err != nil {
					logger.Error("failed to evaluate journey entrance rule", zap.Error(err))
					return err
				}

				if !match {
					continue
				}
			}

			logger.Info("triggering journey entrance step for organization users", zap.Stringer("journey_id", dep.JourneyID), zap.Stringer("step_id", dep.StepID))

			multiple := entrance.Multiple != nil && *entrance.Multiple
			concurrent := entrance.Concurrent != nil && *entrance.Concurrent

			data, err := json.Marshal(map[string]any{"data": event.Data})
			if err != nil {
				logger.Error("failed to marshal journey entry data", zap.Error(err))
				return err
			}

			scanner := func(userID uuid.UUID) error {
				eligible, err := jrny.CheckEntryEligibility(ctx, dep.JourneyID, userID, dep.ExternalID, multiple, concurrent)
				if err != nil {
					logger.Error("failed to check journey entry eligibility", zap.Error(err), zap.Stringer("user_id", userID))
					return err
				}

				if !eligible {
					logger.Info("user not eligible to enter journey", zap.Stringer("journey_id", dep.JourneyID), zap.Stringer("user_id", userID))
					metrics.JourneyEntranceRejectionsTotal.WithLabelValues(event.ProjectID.String(), "not_eligible").Inc()
					return nil
				}

				now := time.Now()
				result := journey.JourneyUserState{
					JourneyID:       dep.JourneyID,
					JourneyEntryID:  uuid.New(),
					UserID:          userID,
					ExternalStepID:  dep.ExternalID,
					PinnedVersionID: &dep.VersionID,
					Data:            json.RawMessage(data),
					CompletedAt:     &now,
				}

				_, err = jrny.CreateUserJourneyState(ctx, result)
				if err != nil {
					logger.Error("failed to create journey user state", zap.Error(err), zap.Stringer("user_id", userID))
					return err
				}

				metrics.JourneyEntrancesTotal.WithLabelValues(event.ProjectID.String()).Inc()
				metrics.EventsJourneyTriggersTotal.WithLabelValues("organization").Inc()

				for _, child := range dep.Children {
					stepType, err := jrny.GetStepType(ctx, dep.VersionID, child.ChildExternalID)
					if err != nil {
						logger.Error("failed to get step type for child", zap.Error(err))
						continue
					}

					step := JourneyStep{
						ProjectID:      event.ProjectID,
						JourneyID:      dep.JourneyID,
						JourneyEntryID: result.JourneyEntryID,
						VersionID:      &dep.VersionID,
						ExternalStepID: child.ChildExternalID,
						UserID:         userID,
						StepType:       stepType,
					}

					err = pub.Publish(ctx, schemas.JourneysAdvance(event.ProjectID, dep.JourneyID, userID), step)
					if err != nil {
						logger.Error("failed to publish journey state", zap.Error(err), zap.Stringer("user_id", userID))
						return err
					}
				}

				return nil
			}

			_, err = usrs.ScanOrganizationMembers(ctx, event.ProjectID, event.OrganizationID, entrance.UserRule, scanner)
			if err != nil {
				logger.Error("failed to scan organization members", zap.Error(err))
				return err
			}

			logger.Info("completed triggering journey entrance step", zap.Stringer("journey_id", dep.JourneyID))
		}

		return nil
	}
}

// OrganizationEventSchemasHandler creates a handler that extracts and stores organization event schema information.
func OrganizationEventSchemasHandler(logger *zap.Logger, usrs *subjects.State) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		event := schemas.OrganizationEvent{}
		err := json.Unmarshal(msg.Data(), &event)
		if err != nil {
			logger.Error("failed to unmarshal organization event message", zap.Error(err))
			return Permanent(err)
		}

		logger.Info("incoming organization event schema", zap.Stringer("event_id", event.ID), zap.Stringer("project_id", event.ProjectID))

		paths := rules.ParsePaths(event.Data)
		err = usrs.UpsertEventSchema(ctx, event.ProjectID, event.ID, paths)
		if err != nil {
			logger.Error("failed to upsert event schema", zap.Error(err))
			return err
		}

		logger.Info("organization event schema processed successfully", zap.Stringer("event_id", event.ID))
		return nil
	}
}
