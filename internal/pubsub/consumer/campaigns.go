package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/lunogram/platform/pkg/modules"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// CampaignsSendHandler renders and persists a campaign send as an inbox
// message. After the inbox row is committed, the handler publishes the
// message to the inbox handler for dispatch. Rate limiting, sent_at
// tracking, and the inbox.message.sent lifecycle event are owned by the
// inbox layer.
func CampaignsSendHandler(logger *zap.Logger, db *sqlx.DB, mgmt *management.State, usrs *subjects.State, renderer *pubsub.EmailRenderer, pub pubsub.Publisher, publicURL string, linkKey []byte, trackingURL string) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		var event schemas.SendCampaign
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			logger.Error("failed to unmarshal send campaign message", zap.Error(err))
			return Permanent(err)
		}

		logger := logger.With(
			zap.Stringer("project_id", event.ProjectID),
			zap.Stringer("campaign_id", event.CampaignID),
			zap.Stringer("user_id", event.UserID),
		)
		logger.Info("processing send campaign message")

		campaign, err := mgmt.GetCampaign(ctx, event.ProjectID, event.CampaignID)
		if err != nil {
			logger.Error("failed to get campaign", zap.Error(err))
			return err
		}

		// NOTE: skip non-transactional campaigns when the user has unsubscribed.
		if !campaign.Transactional && campaign.SubscriptionID != nil {
			unsubscribed, err := mgmt.IsUserUnsubscribed(ctx, event.UserID, *campaign.SubscriptionID)
			if err != nil {
				logger.Error("failed to check subscription status", zap.Error(err))
				return err
			}
			if unsubscribed {
				logger.Info("skipping send, user has unsubscribed", zap.Stringer("subscription_id", *campaign.SubscriptionID))
				return nil
			}
		}

		project, err := mgmt.GetProject(ctx, event.ProjectID)
		if err != nil {
			logger.Error("failed to get project", zap.Error(err))
			return err
		}

		user, err := usrs.GetUser(ctx, event.ProjectID, event.UserID)
		if err != nil {
			logger.Error("failed to get user", zap.Error(err))
			return err
		}

		rendered, err := renderCampaignInboxMessages(ctx, logger, mgmt, usrs, renderer, publicURL, linkKey, trackingURL, event, campaign, project, user)
		if err != nil {
			logger.Error("failed to render campaign inbox message", zap.Error(err))
			return err
		}

		for _, item := range rendered {
			message, err := createCampaignInboxMessage(ctx, db, logger, event, item)
			if errors.Is(err, sql.ErrNoRows) {
				logger.Info("campaign inbox message already exists")
				continue
			}
			if err != nil {
				return err
			}

			// Delegate dispatch to the inbox handler. The inbox handler
			// owns rate limiting, sent_at tracking, and the
			// inbox.message.sent lifecycle event.
			inboxEvent := schemas.InboxMessage{
				ProjectID:        event.ProjectID,
				MessageID:        message.ID,
				SubjectID:        event.UserID,
				Channel:          string(message.Channel),
				SenderIdentityID: message.SenderIdentityID,
				CampaignID:       message.CampaignID,
				BroadcastID:      message.BroadcastID,
			}
			if err := pub.Publish(ctx, schemas.UserInboxProcess(event.ProjectID), inboxEvent); err != nil {
				return fmt.Errorf("publish campaign inbox message for dispatch: %w", err)
			}
		}

		return nil
	}
}

// createCampaignInboxMessage persists the rendered campaign output
// as an inbox row. Lifecycle events and dispatch are owned by the inbox
// handler that receives the message after this function returns.
// (subject, title, body, html, ...) lives in Content; provenance
// (template_id, campaign_id, broadcast_id, journey_*) lives in Data so future
// audits do not have to re-derive it from the external_id key.
func createCampaignInboxMessage(ctx context.Context, db *sqlx.DB, logger *zap.Logger, event schemas.SendCampaign, item renderedCampaignInboxMessage) (*subjects.InboxMessage, error) {
	providerPart := "multi"
	if item.SenderIdentityID != nil {
		providerPart = item.SenderIdentityID.String()
	}
	source, externalID := event.InboxOrigin(providerPart)

	provenance := map[string]any{
		"template_id": item.TemplateID.String(),
		"campaign_id": event.CampaignID.String(),
	}
	if event.BroadcastID != nil {
		provenance["broadcast_id"] = event.BroadcastID.String()
	}
	if event.Data != nil {
		if event.Data.JourneyID != nil {
			provenance["journey_id"] = event.Data.JourneyID.String()
		}
		if event.Data.JourneyEntryID != nil {
			provenance["journey_entry_id"] = event.Data.JourneyEntryID.String()
		}
		if event.Data.JourneyStepID != nil {
			provenance["journey_step_id"] = *event.Data.JourneyStepID
		}
	}

	data, err := json.Marshal(provenance)
	if err != nil {
		return nil, Permanent(fmt.Errorf("marshal inbox message data: %w", err))
	}

	tags := []string{"campaign"}
	if source != schemas.InboxSourceCampaign {
		tags = append([]string{source}, tags...)
	}

	params := subjects.InboxMessageParams{
		ExternalID:       &externalID,
		Channel:          modules.Channel(item.Channel),
		SenderIdentityID: item.SenderIdentityID,
		CampaignID:       &event.CampaignID,
		BroadcastID:      event.BroadcastID,
		Content:          item.RenderedPayload,
		Data:             data,
		Tags:             tags,
		Priority:         ptr.To(int16(3)),
		Source:           &source,
	}

	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
				logger.Error("failed to rollback tx", zap.Error(rbErr))
			}
		}
	}()

	inbox := subjects.NewInboxStore(tx)
	message, err := inbox.CreateUserInboxMessage(ctx, event.ProjectID, event.UserID, params)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return message, nil
}
