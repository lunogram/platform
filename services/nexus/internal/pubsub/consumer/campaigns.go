package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/pkg/modules/providers"
	"github.com/lunogram/platform/services/nexus/internal/providers/channels"
	"github.com/lunogram/platform/services/nexus/internal/pubsub/schemas"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"

	internalProviders "github.com/lunogram/platform/services/nexus/internal/providers"
)

func CampaignsSendHandler(logger *zap.Logger, db *sqlx.DB, registry *internalProviders.Registry) HandlerFunc {
	campaigns := store.NewCampaignsStore(db)
	users := store.NewUsersStore(db)
	devices := store.NewDevicesStore(db)

	return func(ctx context.Context, msg jetstream.Msg) error {
		var event schemas.SendCampaign
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			logger.Error("failed to unmarshal send campaign message", zap.Error(err))
			return err
		}

		logger = logger.With(zap.String("project_id", event.ProjectID.String()), zap.String("campaign_id", event.CampaignID.String()), zap.String("user_id", event.UserID.String()))
		logger.Info("processing send campaign message")

		campaign, err := campaigns.GetCampaign(ctx, event.ProjectID, event.CampaignID)
		if err != nil {
			logger.Error("failed to get campaign", zap.Error(err))
			return err
		}

		user, err := users.GetUser(ctx, event.ProjectID, event.UserID)
		if err != nil {
			logger.Error("failed to get user", zap.Error(err))
			return err
		}

		provider, exists := registry.Get(campaign.Provider.Module)
		if !exists {
			logger.Error("provider module not found", zap.String("module", campaign.Provider.Module))
			return fmt.Errorf("module %s not found", campaign.Provider.Module)
		}

		var config map[string]any
		if err := json.Unmarshal(campaign.Provider.Data, &config); err != nil {
			logger.Error("failed to unmarshal provider config", zap.Error(err))
			return err
		}

		var opts *channels.ComposeOptions
		if providers.Channel(campaign.Channel) == providers.ChannelPush {
			userDevices, err := devices.ListDevicesByUser(ctx, event.ProjectID, event.UserID)
			if err != nil {
				logger.Error("failed to get user devices", zap.Error(err))
				return err
			}
			opts = &channels.ComposeOptions{Devices: userDevices}
		}

		request, err := channels.Compose(providers.Channel(campaign.Channel), config, campaign.Templates[0], user, opts)
		if err != nil {
			logger.Error("failed to compose request", zap.Error(err))
			return err
		}

		response, err := provider.Send(ctx, request)
		if err != nil {
			logger.Error("failed to send via provider", zap.Error(err))
			return err
		}

		logger.Info("campaign sent successfully", zap.String("status", response.Status), zap.String("id", response.ID))
		return nil
	}
}
