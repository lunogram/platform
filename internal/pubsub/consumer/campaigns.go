package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/lunogram/platform/internal/providers/channels"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/render"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/lunogram/platform/pkg/modules/providers"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"

	internalProviders "github.com/lunogram/platform/internal/providers"
)

// userToMap converts a User to a map suitable for Liquid template rendering.
// The result can be used as the "user" key in the render context so that
// {{ user.email }}, {{ user.data.first_name }} etc. work in templates.
func userToMap(user *subjects.User) map[string]any {
	m := map[string]any{
		"id": user.ID.String(),
	}

	if user.Email != nil {
		m["email"] = *user.Email
	}
	if user.Phone != nil {
		m["phone"] = *user.Phone
	}
	if user.ExternalID != nil {
		m["external_id"] = *user.ExternalID
	}
	if user.AnonymousID != nil {
		m["anonymous_id"] = *user.AnonymousID
	}
	if user.Timezone != nil {
		m["timezone"] = *user.Timezone
	}
	if user.Locale != nil {
		m["locale"] = *user.Locale
	}

	if user.Data != nil {
		var data map[string]any
		if err := json.Unmarshal(user.Data, &data); err == nil {
			m["data"] = data
		}
	}

	return m
}

// selectTemplate picks the best template for a user based on locale.
// Priority: user's locale → project's default locale → first template.
func selectTemplate(templates management.Templates, user *subjects.User, project *management.Project) management.Template {
	if len(templates) == 1 {
		return templates[0]
	}

	byLocale := make(map[string]management.Template, len(templates))
	for _, t := range templates {
		byLocale[t.Locale] = t
	}

	if user.Locale != nil {
		if t, ok := byLocale[*user.Locale]; ok {
			return t
		}
	}

	if t, ok := byLocale[project.Locale]; ok {
		return t
	}

	return templates[0]
}

// buildRenderData builds the Liquid render context for a campaign send.
// Merge order (lowest → highest priority):
//  1. Campaign variable defaults (top-level string keys)
//  2. SendCampaign.Data from journey/API (overrides defaults)
//  3. "user" key (always set, cannot be overridden)
//  4. System-generated values (campaign, now, URLs)
func buildRenderData(publicURL string, user *subjects.User, campaign *management.Campaign, eventData map[string]string) map[string]any {
	data := make(map[string]any)

	for _, v := range campaign.Variables.Data {
		if v.Default != nil {
			data[v.Name] = *v.Default
		}
	}

	for k, v := range eventData {
		data[k] = v
	}

	data["user"] = userToMap(user)
	data["campaign"] = map[string]any{
		"name": campaign.Name,
	}

	data["now"] = time.Now()

	base := strings.TrimRight(publicURL, "/")
	data["preferences_url"] = fmt.Sprintf("%s/preferences/%s/%s", base, campaign.ProjectID, user.ID)

	if campaign.SubscriptionID != nil {
		unsubLink := url.Values{}
		unsubLink.Set("u", user.ID.String())
		unsubLink.Set("c", campaign.ID.String())
		data["unsubscribe_url"] = fmt.Sprintf("%s/unsubscribe/email?link=%s", base, url.QueryEscape("?"+unsubLink.Encode()))
	}

	return data
}

func CampaignsSendHandler(logger *zap.Logger, mgmt *management.State, usrs *subjects.State, registry *internalProviders.Registry, publicURL string) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		var event schemas.SendCampaign
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			logger.Error("failed to unmarshal send campaign message", zap.Error(err))
			return err
		}

		logger = logger.With(zap.String("project_id", event.ProjectID.String()), zap.String("campaign_id", event.CampaignID.String()), zap.String("user_id", event.UserID.String()))
		logger.Info("processing send campaign message")

		campaign, err := mgmt.GetCampaign(ctx, event.ProjectID, event.CampaignID)
		if err != nil {
			logger.Error("failed to get campaign", zap.Error(err))
			return err
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

		data := buildRenderData(publicURL, user, campaign, event.Data)
		template := selectTemplate(campaign.Templates, user, project)
		template.Data, err = render.RenderJSON(template.Data, data)
		if err != nil {
			logger.Error("failed to render template data", zap.Error(err))
			return err
		}

		var opts *channels.ComposeOptions
		if providers.Channel(campaign.Channel) == providers.ChannelPush {
			userDevices, err := usrs.ListDevicesByUser(ctx, event.ProjectID, event.UserID)
			if err != nil {
				logger.Error("failed to get user devices", zap.Error(err))
				return err
			}
			opts = &channels.ComposeOptions{Devices: userDevices}
		}

		request, err := channels.Compose(providers.Channel(campaign.Channel), config, template, user, opts)
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
