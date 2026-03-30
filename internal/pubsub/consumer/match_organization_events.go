package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/node/metrics"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// MatchOrganizationEventsHandler creates a handler that resolves a JSONB match
// filter into individual organization IDs and publishes an OrganizationEvent
// for each matched organization.
func MatchOrganizationEventsHandler(logger *zap.Logger, usrs *subjects.State, pub pubsub.Publisher) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		var event schemas.MatchOrganizationEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			logger.Error("failed to unmarshal match organization event", zap.Error(err))
			return Permanent(err)
		}

		logger.Info("processing match organization event", zap.String("name", event.Name), zap.Stringer("project_id", event.ProjectID), zap.Any("match", event.Match))

		// Derive a stable origin ID from the incoming NATS message so that
		// redeliveries of the same match event produce identical publish
		// dedup keys and don't create duplicate organization events.
		origin := msg.Headers().Get(MsgIDHeader)
		if origin == "" {
			origin = uuid.New().String()
		}

		organizations, err := usrs.ScanOrganizationsMatchingData(ctx, event.ProjectID, event.Match, func(orgID uuid.UUID) error {
			orgEvent := schemas.OrganizationEvent{
				ProjectID:      event.ProjectID,
				Name:           event.Name,
				OrganizationID: orgID,
				Data:           event.Data,
			}

			id := fmt.Sprintf("match-org:%s:%s", origin, orgID)
			return pub.Publish(ctx, schemas.OrganizationEventsProcess(event.ProjectID), orgEvent, pubsub.WithMsgID(id))
		})
		if err != nil {
			logger.Error("failed to scan matching organizations", zap.Error(err))
			return err
		}

		logger.Info("match organization event processed",
			zap.String("name", event.Name),
			zap.Stringer("project_id", event.ProjectID),
			zap.Int("matched_organizations", organizations),
		)

		metrics.MatchEventsProcessedTotal.WithLabelValues("organization").Inc()
		metrics.MatchEventsMatchedTotal.WithLabelValues("organization").Add(float64(organizations))
		return nil
	}
}
