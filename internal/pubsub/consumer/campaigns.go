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

	if user != nil && user.Locale != nil {
		if t, ok := byLocale[*user.Locale]; ok {
			return t
		}
	}

	if project != nil {
		if t, ok := byLocale[project.Locale]; ok {
			return t
		}
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

	baseURL := strings.TrimSuffix(publicURL, "/")
	data["preferences_url"] = fmt.Sprintf("%s/preferences/%s/%s", baseURL, campaign.ProjectID, user.ID)

	if !campaign.Transactional && campaign.SubscriptionID != nil {
		unsubLink := url.Values{}
		unsubLink.Set("u", user.ID.String())
		unsubLink.Set("c", campaign.ID.String())
		data["unsubscribe_url"] = fmt.Sprintf("%s/unsubscribe/email?link=%s", baseURL, url.QueryEscape("?"+unsubLink.Encode()))
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

		logger := logger.With(zap.Stringer("project_id", event.ProjectID), zap.Stringer("campaign_id", event.CampaignID), zap.Stringer("user_id", event.UserID))

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

		template := selectTemplate(campaign.Templates, user, project)

		opts := &channels.ComposeOptions{}
		channel := providers.Channel(campaign.Channel)

		var (
			templateSender *management.SenderIdentity
			activeProvider *management.Provider
			pushDispatches []pushDispatch
		)

		if channel == providers.ChannelPush {
			pushDispatches, err = resolvePushDispatches(ctx, mgmt, event.ProjectID, event.UserID, usrs)
			if err != nil {
				logger.Error("failed to resolve push providers", zap.Error(err))
				return err
			}
		} else {
			activeProvider, templateSender, err = resolveMessageProvider(ctx, mgmt, event.ProjectID, template)
			if err != nil {
				logger.Error("failed to resolve provider", zap.Error(err))
				return err
			}
		}

		data := buildRenderData(publicURL, user, campaign, event.Data)

		// Channel-specific template rendering:
		// - Email: compile/render React Email body via Deno, then Liquid-render metadata (subject, from, etc.)
		// - SMS, Push: Liquid rendering for the entire template
		switch channel {
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

		referenceType := ""
		referenceID := ""

		if channel == providers.ChannelPush {
			var (
				successCount  int
				firstSuccess  *providers.SendResponse
				firstProvider *management.Provider
				firstErr      error
			)

			for _, dispatch := range pushDispatches {
				provider, exists := registry.Get(dispatch.Provider.Module)
				if !exists {
					err := Permanentf("module %s not found", dispatch.Provider.Module)
					logger.Error("provider module not found", zap.String("module", dispatch.Provider.Module), zap.Error(err))
					if firstErr == nil {
						firstErr = err
					}
					continue
				}

				rateLimit := providers.ResolveLimit(
					providers.ProviderKey(dispatch.Provider.ID),
					provider.Manifest().Spec.RateLimit,
					dispatch.Provider.RateLimit,
					dispatch.Provider.RateInterval,
				)

				err = limiter.Throttle(ctx, logger, rateLimit, msg)
				if err != nil {
					logger.Warn("failed to acquire rate limit for push provider", zap.Stringer("provider_id", dispatch.Provider.ID), zap.Error(err))
					if firstErr == nil {
						firstErr = err
					}
					continue
				}

				var config map[string]any
				if err := json.Unmarshal(dispatch.Provider.Data, &config); err != nil {
					logger.Error("failed to unmarshal provider config", zap.Error(err))
					if firstErr == nil {
						firstErr = Permanent(err)
					}
					continue
				}

				opts.Devices = dispatch.Devices
				opts.LinkWrap = nil
				if len(linkKey) > 0 && trackingURL != "" && dispatch.Provider.LinkWrap {
					opts.LinkWrap = &channels.LinkWrapConfig{
						Key:         linkKey,
						TrackingURL: trackingURL,
						ProjectID:   event.ProjectID,
						CampaignID:  event.CampaignID,
						UserID:      event.UserID,
					}
				}

				request, err := channels.Compose(ctx, logger, channel, nil, nil, config, template, user, opts)
				if err != nil {
					logger.Error("failed to compose push request", zap.Stringer("provider_id", dispatch.Provider.ID), zap.Error(err))
					if firstErr == nil {
						firstErr = Permanent(err)
					}
					continue
				}

				response, err := provider.Send(ctx, request)
				if err != nil {
					logger.Error("failed to send push via provider", zap.Stringer("provider_id", dispatch.Provider.ID), zap.Error(err))

					var providerErr *wasmProviders.ProviderError
					if errors.As(err, &providerErr) && providerErr.IsPermanent() {
						err = Permanent(err)
					}

					if firstErr == nil {
						firstErr = err
					}
					continue
				}

				successCount++
				if firstSuccess == nil {
					firstSuccess = response
					firstProvider = dispatch.Provider
				}
			}

			if successCount == 0 {
				limit, is := IsRateLimited(firstErr)
				if is {
					subject := schemas.CampaignsSend(event.ProjectID, event.CampaignID)
					at := pubsub.At(time.Now().Add(limit.RetryAfter))
					if err := pub.Publish(ctx, subject, event, at); err != nil {
						return fmt.Errorf("schedule rate-limited message: %w", err)
					}
				}

				if firstErr != nil {
					return firstErr
				}

				return Permanentf("failed to send push notification: no providers were resolved")
			}

			referenceType = "push_multi"
			if successCount == 1 && firstProvider != nil {
				referenceType = firstProvider.Module
			}
			if firstSuccess != nil {
				referenceID = firstSuccess.ID
			}
			if referenceID == "" {
				referenceID = event.UserID.String()
			}

			logger.Info("push notification sent", zap.Int("providers_success", successCount), zap.Int("providers_total", len(pushDispatches)))
		} else {
			provider, exists := registry.Get(activeProvider.Module)
			if !exists {
				logger.Error("provider module not found", zap.String("module", activeProvider.Module))
				return Permanentf("module %s not found", activeProvider.Module)
			}

			rateLimit := providers.ResolveLimit(
				providers.ProviderKey(activeProvider.ID),
				provider.Manifest().Spec.RateLimit,
				activeProvider.RateLimit,
				activeProvider.RateInterval,
			)

			err = limiter.Throttle(ctx, logger, rateLimit, msg)
			limit, is := IsRateLimited(err)
			if is {
				subject := schemas.CampaignsSend(event.ProjectID, event.CampaignID)
				at := pubsub.At(time.Now().Add(limit.RetryAfter))
				if err := pub.Publish(ctx, subject, event, at); err != nil {
					return fmt.Errorf("schedule rate-limited message: %w", err)
				}
				return err
			}

			if err != nil {
				return err
			}

			var config map[string]any
			if err := json.Unmarshal(activeProvider.Data, &config); err != nil {
				logger.Error("failed to unmarshal provider config", zap.Error(err))
				return Permanent(err)
			}

			// Resolve provider default_from.
			providerDefaultSender, err := channels.ResolveProviderDefaultFrom(ctx, mgmt.SenderIdentitiesStore, event.ProjectID, config)
			if err != nil {
				logger.Error("failed to resolve provider default from", zap.Error(err))
				return err
			}

			opts.LinkWrap = nil
			if len(linkKey) > 0 && trackingURL != "" && activeProvider.LinkWrap {
				opts.LinkWrap = &channels.LinkWrapConfig{
					Key:         linkKey,
					TrackingURL: trackingURL,
					ProjectID:   event.ProjectID,
					CampaignID:  event.CampaignID,
					UserID:      event.UserID,
				}
			}

			request, err := channels.Compose(ctx, logger, channel, templateSender, providerDefaultSender, config, template, user, opts)
			if err != nil {
				logger.Error("failed to compose request", zap.Error(err))
				// Compose errors are configuration/validation issues (e.g. "user has
				// no email address") that will not resolve on retry.
				return Permanent(err)
			}

			response, err := provider.Send(ctx, request)
			if err != nil {
				logger.Error("failed to send via provider", zap.Error(err))

				var providerErr *wasmProviders.ProviderError
				if errors.As(err, &providerErr) && providerErr.IsPermanent() {
					return Permanent(err)
				}
				return err
			}

			referenceType = activeProvider.Module
			referenceID = response.ID
			if referenceID == "" {
				referenceID = event.UserID.String()
			}

			logger.Info("campaign sent successfully", zap.String("status", response.Status), zap.String("id", response.ID))
		}

		now := time.Now()
		state := subjects.CampaignSendStateSent

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

// resolveMessageProvider resolves the provider for email/SMS channels by
// looking up the template's sender identity. It returns both the provider and
// the sender identity so the caller can reuse it for composing the message.
func resolveMessageProvider(ctx context.Context, mgmt *management.State, projectID uuid.UUID, template management.Template) (*management.Provider, *management.SenderIdentity, error) {
	if template.SenderIdentityID == nil {
		return nil, nil, Permanentf("template %s has no sender identity configured", template.ID)
	}

	senderIdentity, err := mgmt.SenderIdentitiesStore.GetSenderIdentity(ctx, projectID, *template.SenderIdentityID)
	if err != nil {
		return nil, nil, fmt.Errorf("get sender identity: %w", err)
	}

	provider, err := mgmt.ProvidersStore.GetProvider(ctx, senderIdentity.ProviderID)
	if err != nil {
		return nil, nil, fmt.Errorf("get provider from sender identity: %w", err)
	}

	return provider, senderIdentity, nil
}

type pushDispatch struct {
	Provider *management.Provider
	Devices  subjects.Devices
}

// resolvePushDispatches groups user devices by configured platform provider
// and returns one dispatch entry per provider so push sends can fan out.
func resolvePushDispatches(ctx context.Context, mgmt *management.State, projectID, userID uuid.UUID, usrs *subjects.State) ([]pushDispatch, error) {
	devices, err := usrs.ListDevicesByUserWithConfig(ctx, projectID, userID)
	if err != nil {
		return nil, fmt.Errorf("list user devices: %w", err)
	}

	if len(devices) == 0 {
		return nil, Permanentf("user %s has no push-enabled devices", userID)
	}

	pushProviders, err := mgmt.ProjectPushProvidersStore.ListProjectPushProviders(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list project push providers: %w", err)
	}

	if len(pushProviders) == 0 {
		return nil, Permanentf("project %s has no push providers configured", projectID)
	}

	byPlatform := make(map[string]uuid.UUID, len(pushProviders))
	for _, pp := range pushProviders {
		byPlatform[pp.Platform] = pp.ProviderID
	}

	grouped := make(map[uuid.UUID]subjects.Devices)

	for _, device := range devices {
		platform := platformForDevice(device)
		if platform == "" {
			continue
		}

		if providerID, ok := byPlatform[platform]; ok {
			grouped[providerID] = append(grouped[providerID], device)
		}
	}

	if len(grouped) == 0 {
		fallbackProviderID := pushProviders[0].ProviderID
		grouped[fallbackProviderID] = append(grouped[fallbackProviderID], devices...)
	}

	dispatches := make([]pushDispatch, 0, len(grouped))
	for providerID, providerDevices := range grouped {
		provider, err := mgmt.ProvidersStore.GetProvider(ctx, providerID)
		if err != nil {
			return nil, fmt.Errorf("get push provider %s: %w", providerID, err)
		}

		dispatches = append(dispatches, pushDispatch{
			Provider: provider,
			Devices:  providerDevices,
		})
	}

	if len(dispatches) == 0 {
		return nil, Permanentf("project %s has no valid push providers configured", projectID)
	}

	return dispatches, nil
}

func platformForDevice(device subjects.Device) string {
	if device.OS != nil {
		switch *device.OS {
		case management.PlatformIOS, management.PlatformAndroid, management.PlatformWeb:
			return *device.OS
		}
	}

	if device.Config == nil {
		return ""
	}

	switch device.Config.Type {
	case subjects.PushConfigTypeAPNs:
		return management.PlatformIOS
	case subjects.PushConfigTypeFCM:
		return management.PlatformAndroid
	case subjects.PushConfigTypeWebPush:
		return management.PlatformWeb
	default:
		return ""
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
