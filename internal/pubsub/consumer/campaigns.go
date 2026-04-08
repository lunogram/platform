package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/providers/channels"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/render"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	wasmProviders "github.com/lunogram/platform/internal/wasm/providers"
	providers "github.com/lunogram/platform/pkg/modules/providers"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"

	internalProviders "github.com/lunogram/platform/internal/providers"
)

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

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
	if r := user.ExternalIDBySource("default"); r != nil {
		m["external_id"] = r.ExternalID
	}
	if r := user.ExternalIDBySource("anonymous"); r != nil {
		m["anonymous_id"] = r.ExternalID
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
//  1. Campaign variable defaults (under "campaign" key)
//  2. SendCampaign.Data from journey/API (overrides defaults)
//  3. "user" key (always set, cannot be overridden)
//  4. System-generated values (now, URLs)
func buildRenderData(publicURL string, user *subjects.User, campaign *management.Campaign, eventData map[string]string) map[string]any {
	data := make(map[string]any)

	campaignVars := make(map[string]any)
	for _, v := range campaign.Variables.Data {
		if v.Default != nil {
			campaignVars[v.Name] = *v.Default
		}
	}

	// Event data can override campaign variable defaults
	for k, v := range eventData {
		campaignVars[k] = v
	}

	data["campaign"] = campaignVars
	data["user"] = userToMap(user)

	data["now"] = time.Now()

	data["preferences_url"] = fmt.Sprintf("%s/preferences/%s/%s", publicURL, campaign.ProjectID, user.ID)

	if campaign.SubscriptionID != nil {
		unsubLink := url.Values{}
		unsubLink.Set("u", user.ID.String())
		unsubLink.Set("c", campaign.ID.String())
		data["unsubscribe_url"] = fmt.Sprintf("%s/unsubscribe/email?link=%s", publicURL, url.QueryEscape("?"+unsubLink.Encode()))
	}

	return data
}

func CampaignsSendHandler(logger *zap.Logger, mgmt *management.State, usrs *subjects.State, registry *internalProviders.Registry, renderer *pubsub.EmailRenderer, pub pubsub.Publisher, limiter *Limiter, publicURL string, linkKey []byte, trackingURL string) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) (err error) {
		var event schemas.SendCampaign
		err = json.Unmarshal(msg.Data(), &event)
		if err != nil {
			logger.Error("failed to unmarshal send campaign message", zap.Error(err))
			return Permanent(err)
		}

		logger := logger.With(zap.String("project_id", event.ProjectID.String()), zap.String("campaign_id", event.CampaignID.String()), zap.String("user_id", event.UserID.String()))

		err = limiter.Throttle(ctx, logger, event.RateLimit, msg)
		limiter, is := IsRateLimited(err)
		if is {
			subject := schemas.CampaignsSend(event.ProjectID, event.CampaignID)
			at := pubsub.At(time.Now().Add(limiter.RetryAfter))
			if err := pub.Publish(ctx, subject, event, at); err != nil {
				return fmt.Errorf("schedule rate-limited message: %w", err)
			}
			return err
		}

		if err != nil {
			return err
		}

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
			return Permanentf("module %s not found", campaign.Provider.Module)
		}

		var config map[string]any
		if err := json.Unmarshal(campaign.Provider.Data, &config); err != nil {
			logger.Error("failed to unmarshal provider config", zap.Error(err))
			return Permanent(err)
		}

		data := buildRenderData(publicURL, user, campaign, event.Data)
		template := selectTemplate(campaign.Templates, user, project)

		// Channel-specific template rendering:
		// - Email: compile/render React Email body via Deno, then Liquid-render metadata (subject, from, etc.)
		// - SMS, Push: Liquid rendering for the entire template
		switch providers.Channel(campaign.Channel) {
		case providers.ChannelEmail:
			template.Data, err = channels.ComposeEmailTemplateData(ctx, renderer, event.ProjectID, template.Data, data)
			if err != nil {
				logger.Error("failed to compose email template data", zap.Error(err))
				return err
			}

			template.Data, err = render.RenderJSON(template.Data, data)
			if err != nil {
				logger.Error("failed to render template metadata", zap.Error(err))
				return Permanent(err)
			}
		default:
			template.Data, err = render.RenderJSON(template.Data, data)
			if err != nil {
				logger.Error("failed to render template data", zap.Error(err))
				return Permanent(err)
			}
		}

		// Resolve template sender identity if set.
		var templateSender *management.SenderIdentity
		if template.SenderIdentityID != nil {
			templateSender, err = mgmt.SenderIdentitiesStore.GetSenderIdentity(ctx, event.ProjectID, *template.SenderIdentityID)
			if err != nil {
				logger.Error("failed to get template sender identity", zap.Error(err))
				return err
			}
		}

		// Resolve provider default_from.
		providerDefaultSender, err := channels.ResolveProviderDefaultFrom(ctx, mgmt.SenderIdentitiesStore, event.ProjectID, config)
		if err != nil {
			logger.Error("failed to resolve provider default from", zap.Error(err))
			return err
		}

		opts := &channels.ComposeOptions{}

		if len(linkKey) > 0 && trackingURL != "" && campaign.Provider != nil && campaign.Provider.LinkWrap {
			opts.LinkWrap = &channels.LinkWrapConfig{
				Key:         linkKey,
				TrackingURL: trackingURL,
				ProjectID:   event.ProjectID,
				CampaignID:  event.CampaignID,
				UserID:      event.UserID,
			}
		}

		if providers.Channel(campaign.Channel) == providers.ChannelPush {
			userDevices, err := usrs.ListDevicesByUserWithPushConfig(ctx, event.ProjectID, event.UserID)
			if err != nil {
				logger.Error("failed to get user devices", zap.Error(err))
				return err
			}
			opts.Devices = userDevices

			// Fan-out: send to every push provider configured for this project.
			pushProviders, err := mgmt.ListProvidersByChannel(ctx, event.ProjectID, string(providers.ChannelPush))
			if err != nil {
				logger.Error("failed to list push providers", zap.Error(err))
				return err
			}
			if len(pushProviders) == 0 {
				logger.Warn("no push providers configured for project")
				return Permanent(fmt.Errorf("no push providers configured for project %s", event.ProjectID))
			}

			var lastResponse *providers.SendResponse
			for _, pushProvider := range pushProviders {
				pLogger := logger.With(zap.String("push_module", pushProvider.Module))

				pushModule, exists := registry.Get(pushProvider.Module)
				if !exists {
					pLogger.Warn("push provider module not found, skipping")
					continue
				}

				var pushConfig map[string]any
				if err := json.Unmarshal(pushProvider.Data, &pushConfig); err != nil {
					pLogger.Error("failed to unmarshal push provider config", zap.Error(err))
					continue
				}

				req, err := channels.ComposePushForModule(ctx, pushProvider.Module, pushConfig, template, user, userDevices)
				if errors.Is(err, channels.ErrNoTargets) {
					pLogger.Debug("no devices for push module, skipping")
					continue
				}
				if err != nil {
					pLogger.Error("failed to compose push request", zap.Error(err))
					continue
				}

				resp, err := pushModule.Send(ctx, req)
				if err != nil {
					pLogger.Error("failed to send push via provider", zap.Error(err))
					continue
				}

				pLogger.Info("push sent", zap.String("status", resp.Status), zap.String("id", resp.ID))
				if resp.Metadata != nil {
					for key, meta := range resp.Metadata {
						if m, ok := meta.(map[string]any); ok {
							pLogger.Info("push delivery details",
								zap.String("provider", key),
								zap.Int("success_count", toInt(m["success_count"])),
								zap.Int("failure_count", toInt(m["failure_count"])),
								zap.Any("errors", m["errors"]),
							)
						}
					}
				}
				lastResponse = resp
			}

			// Persist one send record for the campaign regardless of how many
			// push providers were used. Use the last successful response for the
			// reference ID, falling back to the campaign's primary provider module.
			now := time.Now()
			state := subjects.CampaignSendStateSent
			referenceType := campaign.Provider.Module
			var referenceID string
			if lastResponse != nil {
				referenceID = lastResponse.ID
			}

			sendRecord := subjects.CampaignSend{
				CampaignID:    event.CampaignID,
				UserID:        event.UserID,
				BroadcastID:   event.BroadcastID,
				State:         &state,
				SentAt:        &now,
				ReferenceType: &referenceType,
				ReferenceID:   referenceID,
			}

			if err := usrs.CampaignSendsStore.InsertCampaignSend(ctx, sendRecord); err != nil {
				logger.Error("failed to insert campaign send record", zap.Error(err))
				return fmt.Errorf("failed to insert campaign send record: %w", err)
			}

			logger.Info("push campaign sent successfully")
		} else {
			request, err := channels.Compose(ctx, logger, providers.Channel(campaign.Channel), templateSender, providerDefaultSender, config, template, user, opts)
			if err != nil {
				logger.Error("failed to compose request", zap.Error(err))
				// Compose errors are configuration/validation issues (e.g. "user has no email address",
				// "no from address specified") that will not resolve on retry.
				return Permanent(err)
			}

			response, err := provider.Send(ctx, request)
			if err != nil {
				logger.Error("failed to send via provider", zap.Error(err))

				// TEMPORARY: wasm crashes are not retryable — stop the storm
				if strings.Contains(err.Error(), "wasm error:") {
					logger.Warn("wasm crash detected, marking as permanent failure")
					return Permanent(err)
				}

				var providerErr *wasmProviders.ProviderError
				if errors.As(err, &providerErr) && providerErr.IsPermanent() {
					return Permanent(err)
				}
				return err
			}

			now := time.Now()
			state := subjects.CampaignSendStateSent
			referenceType := campaign.Provider.Module
			referenceID := response.ID

			sendRecord := subjects.CampaignSend{
				CampaignID:    event.CampaignID,
				UserID:        event.UserID,
				BroadcastID:   event.BroadcastID,
				State:         &state,
				SentAt:        &now,
				ReferenceType: &referenceType,
				ReferenceID:   referenceID,
			}

			if err := usrs.CampaignSendsStore.InsertCampaignSend(ctx, sendRecord); err != nil {
				logger.Error("failed to insert campaign send record", zap.Error(err))
				return fmt.Errorf("failed to insert campaign send record: %w", err)
			}

			logger.Info("campaign sent successfully", zap.String("status", response.Status), zap.String("id", response.ID))
		}

		// If this send belongs to a broadcast, check whether all messages
		// have now been delivered. The batch handler keeps the broadcast in
		// "sending" state after publishing all SendCampaign messages; we
		// transition to "completed" here once the last one is actually sent.
		if event.BroadcastID != nil {
			if err := checkBroadcastCompletion(ctx, logger, mgmt, usrs, event.ProjectID, *event.BroadcastID); err != nil {
				logger.Error("broadcast completion check failed", zap.Error(err))
			}
		}

		return nil
	}
}

// checkBroadcastCompletion determines whether all sends for a broadcast have
// been delivered and, if so, transitions the broadcast to the completed state.
func checkBroadcastCompletion(ctx context.Context, logger *zap.Logger, mgmt *management.State, usrs *subjects.State, projectID, broadcastID uuid.UUID) error {
	broadcast, err := mgmt.GetBroadcast(ctx, projectID, broadcastID)
	if err != nil {
		return fmt.Errorf("get broadcast: %w", err)
	}

	if broadcast.State != management.BroadcastStateSending || broadcast.Total <= 0 {
		return nil
	}

	sent, err := usrs.CampaignSendsStore.CountSendsByBroadcastID(ctx, broadcastID)
	if err != nil {
		return fmt.Errorf("count sends: %w", err)
	}

	if sent < broadcast.Total {
		return nil
	}

	transitioned, err := mgmt.BroadcastsStore.TransitionBroadcastState(ctx, projectID, broadcastID, management.BroadcastStateSending, management.BroadcastStateCompleted, broadcast.Total, nil)
	if err != nil {
		return fmt.Errorf("transition broadcast state: %w", err)
	}

	if transitioned {
		logger.Info("broadcast completed", zap.String("broadcast_id", broadcastID.String()), zap.Int("sent", sent), zap.Int("total", broadcast.Total))
	}

	return nil
}
