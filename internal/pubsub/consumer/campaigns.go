package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"

	internalProviders "github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/providers/channels"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/render"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	wasmProviders "github.com/lunogram/platform/internal/wasm/providers"
	providers "github.com/lunogram/platform/pkg/modules/providers"
)

// CampaignsSendHandler returns a HandlerFunc that processes individual
// campaign send messages (one per user).
func CampaignsSendHandler(logger *zap.Logger, mgmt *management.State, usrs *subjects.State, registry *internalProviders.Registry, renderer *pubsub.EmailRenderer, pub pubsub.Publisher, limiter *Limiter, publicURL string, linkKey []byte, trackingURL string) HandlerFunc {
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

		var (
			provider       *management.Provider
			senderIdentity *management.SenderIdentity
			devices        subjects.Devices
		)

		switch providers.Channel(campaign.Channel) {
		case providers.ChannelPush:
			provider, devices, err = resolvePushProvider(ctx, mgmt, event.ProjectID, user, usrs)
		default:
			provider, senderIdentity, err = resolveMessageProvider(ctx, mgmt, event.ProjectID, template)
		}
		if err != nil {
			logger.Error("failed to resolve provider", zap.Error(err))
			return err
		}

		module, exists := registry.Get(provider.Module)
		if !exists {
			return Permanentf("module %s not found", provider.Module)
		}

		rateLimit := providers.ResolveLimit(
			providers.ProviderKey(provider.ID),
			module.Manifest().Spec.RateLimit,
			provider.RateLimit,
			provider.RateInterval,
		)

		err = limiter.Throttle(ctx, logger, rateLimit, msg)
		if limit, is := IsRateLimited(err); is {
			subject := schemas.CampaignsSend(event.ProjectID, event.CampaignID)
			if err := pub.Publish(ctx, subject, event, pubsub.At(time.Now().Add(limit.RetryAfter))); err != nil {
				return fmt.Errorf("schedule rate-limited message: %w", err)
			}
		}
		if err != nil {
			return err
		}

		config, defaultSender, err := resolveProviderConfig(ctx, mgmt, provider, event.ProjectID)
		if err != nil {
			logger.Error("failed to resolve provider config", zap.Error(err))
			return err
		}

		data := buildRenderData(publicURL, user, campaign, event.Data)
		template, err = renderCampaignTemplate(ctx, renderer, event.ProjectID, campaign.Channel, template, data)
		if err != nil {
			logger.Error("failed to render template", zap.Error(err))
			return err
		}

		opts := &channels.ComposeOptions{
			Devices: devices,
		}
		if len(linkKey) > 0 && trackingURL != "" && provider.LinkWrap {
			opts.LinkWrap = &channels.LinkWrapConfig{
				Key:         linkKey,
				TrackingURL: trackingURL,
				ProjectID:   event.ProjectID,
				CampaignID:  event.CampaignID,
				UserID:      event.UserID,
			}
		}

		request, err := channels.Compose(ctx, logger, providers.Channel(campaign.Channel), senderIdentity, defaultSender, config, template, user, opts)
		if err != nil {
			logger.Error("failed to compose request", zap.Error(err))
			// Compose errors are configuration/validation issues (e.g. "user
			// has no email address") that will not resolve on retry.
			return Permanent(err)
		}

		response, err := module.Send(ctx, request)
		if err != nil {
			logger.Error("failed to send via provider", zap.Error(err))
			var providerErr *wasmProviders.ProviderError
			if errors.As(err, &providerErr) && providerErr.IsPermanent() {
				return Permanent(err)
			}
			return err
		}

		now := time.Now()
		state := subjects.CampaignSendStateSent
		referenceType := provider.Module

		err = usrs.CampaignSendsStore.InsertCampaignSend(ctx, subjects.CampaignSend{
			CampaignID:    event.CampaignID,
			UserID:        event.UserID,
			BroadcastID:   event.BroadcastID,
			State:         &state,
			SentAt:        &now,
			ReferenceType: &referenceType,
			ReferenceID:   response.ID,
		})
		if err != nil {
			logger.Error("failed to insert campaign send record", zap.Error(err))
			return fmt.Errorf("insert campaign send record: %w", err)
		}

		logger.Info("campaign sent successfully",
			zap.String("status", response.Status),
			zap.String("id", response.ID),
		)

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

// resolveProviderConfig unmarshals the provider's JSON config and resolves the
// provider-level default sender identity. Called after the rate-limit check so
// that rate-limited messages never pay this cost.
func resolveProviderConfig(ctx context.Context, mgmt *management.State, provider *management.Provider, projectID uuid.UUID) (config map[string]any, defaultSender *management.SenderIdentity, err error) {
	if err = json.Unmarshal(provider.Data, &config); err != nil {
		return nil, nil, Permanent(fmt.Errorf("unmarshal provider config: %w", err))
	}

	defaultSender, err = channels.ResolveProviderDefaultFrom(ctx, mgmt.SenderIdentitiesStore, projectID, config)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve provider default from: %w", err)
	}

	return config, defaultSender, nil
}

// renderCampaignTemplate applies Liquid (and for email, React Email) rendering
// to the template using the provided render data.
func renderCampaignTemplate(ctx context.Context, renderer *pubsub.EmailRenderer, projectID uuid.UUID, channel string, template management.Template, data map[string]any) (management.Template, error) {
	var err error

	if providers.Channel(channel) == providers.ChannelEmail {
		template.Data, err = channels.ComposeEmailTemplateData(ctx, renderer, projectID, template.Data, data)
		if err != nil {
			return template, err // retryable — renderer may be temporarily unavailable
		}
	}

	template.Data, err = render.RenderJSON(template.Data, data)
	if err != nil {
		return template, Permanent(err) // template syntax errors won't resolve on retry
	}

	return template, nil
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

	locales := make(map[string]management.Template, len(templates))
	for _, t := range templates {
		locales[t.Locale] = t
	}

	if user != nil && user.Locale != nil {
		if t, ok := locales[*user.Locale]; ok {
			return t
		}
	}

	if project != nil {
		if t, ok := locales[project.Locale]; ok {
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

	data["preferences_url"] = fmt.Sprintf("%s/preferences/%s/%s", publicURL, campaign.ProjectID, user.ID)

	if campaign.SubscriptionID != nil {
		unsubLink := url.Values{}
		unsubLink.Set("u", user.ID.String())
		unsubLink.Set("c", campaign.ID.String())
		data["unsubscribe_url"] = fmt.Sprintf("%s/unsubscribe/email?link=%s", publicURL, url.QueryEscape("?"+unsubLink.Encode()))
	}

	return data
}

// resolvePushProvider selects the appropriate push provider for a user by
// matching the user's device platforms against the project's configured push
// providers. It returns both the provider and the user's devices so that the
// caller does not need to fetch devices again.
func resolvePushProvider(ctx context.Context, mgmt *management.State, projectID uuid.UUID, user *subjects.User, usrs *subjects.State) (*management.Provider, subjects.Devices, error) {
	devices, err := usrs.ListDevicesByUser(ctx, projectID, user.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("list user devices: %w", err)
	}

	if len(devices) == 0 {
		return nil, nil, Permanentf("user %s has no push-enabled devices", user.ID)
	}

	pushProviders, err := mgmt.ProjectPushProvidersStore.ListProjectPushProviders(ctx, projectID)
	if err != nil {
		return nil, nil, fmt.Errorf("list project push providers: %w", err)
	}

	if len(pushProviders) == 0 {
		return nil, nil, Permanentf("project %s has no push providers configured", projectID)
	}

	// Build a lookup from platform → provider ID.
	byPlatform := make(map[string]uuid.UUID, len(pushProviders))
	for _, pp := range pushProviders {
		byPlatform[pp.Platform] = pp.ProviderID
	}

	// Match the first device OS to a configured push provider.
	for _, device := range devices {
		if device.OS == nil {
			continue
		}
		if providerID, ok := byPlatform[*device.OS]; ok {
			provider, err := mgmt.ProvidersStore.GetProvider(ctx, providerID)
			if err != nil {
				return nil, nil, fmt.Errorf("get push provider %s: %w", providerID, err)
			}
			return provider, devices, nil
		}
	}

	// No device OS matched a configured platform — fall back to the first
	// configured push provider so the send can still proceed.
	provider, err := mgmt.ProvidersStore.GetProvider(ctx, pushProviders[0].ProviderID)
	if err != nil {
		return nil, nil, fmt.Errorf("get fallback push provider: %w", err)
	}
	return provider, devices, nil
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
