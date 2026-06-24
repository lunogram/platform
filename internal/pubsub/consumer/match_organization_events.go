package consumer

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/node/metrics"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	iredis "github.com/lunogram/platform/internal/redis"
	"github.com/lunogram/platform/internal/rules/eval"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// MatchOrganizationEventsHandler creates a handler that resolves a JSONB match
// filter into individual organization IDs, inserts event records for all
// matched organizations in a single database query, and then publishes schema,
// list-recompute, and journey entrance messages. Journey entrance rules are
// evaluated once for all matched organizations and individual JourneyEntrance
// messages are published to NATS for each organization member × matching
// dependency.
func MatchOrganizationEventsHandler(logger *zap.Logger, usrs *subjects.State, jrny *journey.State, pub pubsub.Publisher, schemaCache *iredis.SchemaCache) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		start := time.Now()
		var event schemas.MatchOrganizationEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			logger.Error("failed to unmarshal match organization event", zap.Error(err))
			return Permanent(err)
		}

		logger.Info("processing match organization event",
			zap.String("name", event.Name),
			zap.Stringer("project_id", event.ProjectID),
			zap.Any("match", event.Match),
		)

		// Upsert the event definition once – same for every matched org.
		eventID, err := usrs.UpsertEvent(ctx, event.ProjectID, event.Name, subjects.SubjectTypeOrganization)
		if err != nil {
			logger.Error("failed to upsert event", zap.Error(err))
			return err
		}

		// Insert event records for all matching organizations in a single
		// query and get back the matched IDs.
		orgIDs, err := usrs.InsertMatchingOrganizationEvents(ctx, event.ProjectID, eventID, event.Match, event.Data)
		if err != nil {
			logger.Error("failed to insert matching organization events", zap.Error(err))
			return err
		}

		matched := len(orgIDs)
		logger.Info("matched organizations",
			zap.String("name", event.Name),
			zap.Stringer("project_id", event.ProjectID),
			zap.Int("matched_organizations", matched),
		)

		if matched == 0 {
			metrics.MatchEventsProcessedTotal.WithLabelValues("organization").Inc()
			metrics.MatchEventsMatchedTotal.WithLabelValues("organization").Add(0)
			return nil
		}

		// Build a representative OrganizationEvent so we can reuse the
		// shared dependency helpers for schema and list processing.
		orgEvent := schemas.OrganizationEvent{
			ID:        eventID,
			Name:      event.Name,
			ProjectID: event.ProjectID,
			Data:      event.Data,
		}

		wg, wgCtx := errgroup.WithContext(ctx)
		wg.Go(PublishOrganizationEventSchema(wgCtx, logger, pub, orgEvent, schemaCache))
		wg.Go(PublishOrganizationEventListDependencies(wgCtx, logger, usrs, pub, orgEvent))
		wg.Go(PublishMatchedOrgEntrances(wgCtx, logger, usrs, jrny, pub, eventID, event, orgIDs))

		if err := wg.Wait(); err != nil {
			logger.Error("failed to process dependent events", zap.Error(err))
			return err
		}

		logger.Info("match organization event processed",
			zap.String("name", event.Name),
			zap.Stringer("project_id", event.ProjectID),
			zap.Int("matched_organizations", matched),
		)

		metrics.MatchEventsProcessedTotal.WithLabelValues("organization").Inc()
		metrics.MatchEventsMatchedTotal.WithLabelValues("organization").Add(float64(matched))
		metrics.EventsProcessedTotal.WithLabelValues("organization").Add(float64(matched))
		metrics.EventsProcessingDurationSeconds.WithLabelValues("organization").Observe(time.Since(start).Seconds())
		return nil
	}
}

// PublishMatchedOrgEntrances fetches journey entrance dependencies for the
// given event, evaluates entrance rules once, and publishes a JourneyEntrance
// message for every organization member × qualifying dependency combination.
func PublishMatchedOrgEntrances(ctx context.Context, logger *zap.Logger, usrs *subjects.State, jrny *journey.State, pub pubsub.Publisher, eventID uuid.UUID, event schemas.MatchOrganizationEvent, orgIDs []uuid.UUID) func() error {
	return func() error {
		deps, err := jrny.ListEventJourneyDependencies(ctx, eventID)
		if err != nil {
			logger.Error("failed to list journey event dependencies", zap.Error(err))
			return err
		}

		if len(deps) == 0 {
			return nil
		}

		evaluator := eval.NewEvaluator()

		for _, dep := range deps {
			entrance := oapi.EntranceStepData{}
			if dep.Data != nil {
				if err := json.Unmarshal(*dep.Data, &entrance); err != nil {
					return err
				}
			}

			if rule := entrance.EntranceRule(); rule != nil {
				match, err := evaluator.Evaluate(*rule, event.Data)
				if err != nil {
					logger.Error("failed to evaluate journey entrance rule", zap.Error(err))
					return err
				}

				if !match {
					continue
				}
			}

			childrenJSON, err := json.Marshal(dep.Children)
			if err != nil {
				logger.Error("failed to marshal entrance children", zap.Error(err))
				return err
			}

			multiple := entrance.Multiple
			concurrent := entrance.Concurrent

			for _, orgID := range orgIDs {
				logger.Info("publishing journey entrances for organization members",
					zap.Stringer("journey_id", dep.JourneyID),
					zap.Stringer("organization_id", orgID),
				)

				scanner := func(userID uuid.UUID) error {
					msg := schemas.JourneyEntrance{
						ProjectID:      event.ProjectID,
						JourneyID:      dep.JourneyID,
						VersionID:      dep.VersionID,
						UserID:         userID,
						ExternalStepID: dep.ExternalID,
						Multiple:       multiple,
						Concurrent:     concurrent,
						Data:           event.Data,
						Children:       childrenJSON,
					}

					err := pub.Publish(ctx, schemas.JourneysEntrance(event.ProjectID, dep.JourneyID, userID), msg)
					if err != nil {
						logger.Error("failed to publish journey entrance", zap.Error(err), zap.Stringer("user_id", userID))
						return err
					}

					metrics.EventsJourneyTriggersTotal.WithLabelValues("organization").Inc()
					return nil
				}

				_, err = usrs.ScanOrganizationMembers(ctx, event.ProjectID, orgID, entrance.MemberRule(), scanner)
				if err != nil {
					logger.Error("failed to scan organization members", zap.Error(err), zap.Stringer("organization_id", orgID))
					return err
				}
			}
		}

		return nil
	}
}
