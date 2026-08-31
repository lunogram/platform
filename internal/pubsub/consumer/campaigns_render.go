package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/node/metrics"
	"github.com/lunogram/platform/internal/providers/channels"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/render"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	providers "github.com/lunogram/platform/pkg/modules/providers"
	"go.uber.org/zap"
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

// resolveVariant works out which template variant a send must use.
//
// An explicit variant on the event wins: the publisher - a journey step, or a
// broadcast pinned to one client - has already decided. Otherwise the
// campaign's selector is rendered against the same context the templates
// render against, so a project can drive branding off recipient data such as
// "{{ user.data.tenant }}" without configuring anything per send.
//
// A selector that fails to render resolves to the default variant. The
// expression is customer-authored and a broken one must not take a campaign
// down; the fallback is counted where the template is selected.
func resolveVariant(logger *zap.Logger, campaign *management.Campaign, event schemas.SendCampaign, data map[string]any) string {
	if event.Variant != nil {
		return strings.TrimSpace(*event.Variant)
	}

	if campaign.VariantSelector == nil || *campaign.VariantSelector == "" {
		return ""
	}

	resolved, err := render.RenderString(*campaign.VariantSelector, data)
	if err != nil {
		logger.Warn("failed to render campaign variant selector, using default variant",
			zap.Error(err),
			zap.String("selector", *campaign.VariantSelector))
		return ""
	}

	return strings.TrimSpace(resolved)
}

// selectTemplate picks the template for a send, narrowing by variant before
// applying the locale rules.
//
// Locale priority within a variant is unchanged: user's locale → project's
// default locale → first template. A variant carrying no template for this
// campaign falls back to the default variant rather than failing, because a
// missing white-label template must not stop a message going out; callers see
// that happened by comparing the returned template's Variant against the one
// they asked for.
func selectTemplate(templates management.Templates, variant string, user *subjects.User, project *management.Project) (management.Template, error) {
	if len(templates) == 0 {
		return management.Template{}, Permanentf("campaign has no templates")
	}

	candidates := templatesForVariant(templates, variant)
	if len(candidates) == 0 {
		candidates = templatesForVariant(templates, "")
	}

	// Neither the requested variant nor the default has a template. Every
	// remaining template belongs to some other variant, so there is no
	// on-brand answer left - send one anyway rather than dropping the message,
	// and let the caller's fallback counter surface it.
	if len(candidates) == 0 {
		candidates = templates
	}

	if len(candidates) == 1 {
		return candidates[0], nil
	}

	byLocale := make(map[string]management.Template, len(candidates))
	for _, t := range candidates {
		byLocale[t.Locale] = t
	}

	if user != nil && user.Locale != nil {
		if t, ok := byLocale[*user.Locale]; ok {
			return t, nil
		}
	}

	if project != nil {
		if t, ok := byLocale[project.Locale]; ok {
			return t, nil
		}
	}

	return candidates[0], nil
}

func templatesForVariant(templates management.Templates, variant string) management.Templates {
	matched := make(management.Templates, 0, len(templates))
	for _, template := range templates {
		if template.Variant == variant {
			matched = append(matched, template)
		}
	}
	return matched
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

// renderedCampaignInboxMessage is the in-memory render artifact passed from
// the rendering layer to the inbox creation layer. RenderedPayload becomes
// the persisted InboxMessage.Content as-is.
type renderedCampaignInboxMessage struct {
	Channel          providers.Channel
	SenderIdentityID *uuid.UUID
	TemplateID       uuid.UUID
	// Variant is the variant actually rendered, which is not always the one
	// the send asked for: a missing white-label template falls back to the
	// default. Recording it makes the branding a recipient received auditable
	// without re-deriving it from the template.
	Variant         string
	RenderedPayload json.RawMessage
}

type renderedPushDispatch struct {
	ProviderID uuid.UUID       `json:"provider_id"`
	Payload    json.RawMessage `json:"payload"`
}

// renderedPushPayload is the JSON shape persisted in InboxMessage.Content for
// push messages. Title and Body are hoisted to the top level so that inbox
// list search ('content->>title', 'content->>body') works the same as for
// email/sms payloads.
type renderedPushPayload struct {
	Title      string                 `json:"title"`
	Body       string                 `json:"body,omitempty"`
	Dispatches []renderedPushDispatch `json:"dispatches"`
}

func renderCampaignInboxMessages(ctx context.Context, logger *zap.Logger, mgmt *management.State, usrs *subjects.State, renderer *pubsub.EmailRenderer, publicURL string, linkKey []byte, trackingURL string, event schemas.SendCampaign, campaign *management.Campaign, project *management.Project, user *subjects.User) ([]renderedCampaignInboxMessage, error) {
	channel := providers.Channel(campaign.Channel)
	data := buildRenderData(publicURL, user, campaign, event.Variables)

	variant := resolveVariant(logger, campaign, event, data)
	template, err := selectTemplate(campaign.Templates, variant, user, project)
	if err != nil {
		return nil, err
	}

	outcome := "matched"
	if template.Variant != variant {
		outcome = "fallback"
		logger.Warn("no template for requested variant, falling back",
			zap.String("requested_variant", variant),
			zap.String("selected_variant", template.Variant),
			zap.Stringer("template_id", template.ID))
	}
	metrics.CampaignVariantSelectionsTotal.WithLabelValues(event.ProjectID.String(), outcome).Inc()

	switch channel {
	case providers.ChannelEmail:
		template.Data, err = channels.ComposeEmailTemplateData(ctx, renderer, event.ProjectID, template.Data, data)
		if err != nil {
			return nil, fmt.Errorf("compose email template data: %w", err)
		}

		template.Data, err = render.RenderJSON(template.Data, data)
		if err != nil {
			return nil, Permanent(err)
		}
	case providers.ChannelSMS, providers.ChannelPush, providers.ChannelInbox:
		template.Data, err = render.RenderJSON(template.Data, data)
		if err != nil {
			return nil, Permanent(err)
		}
	default:
		return nil, Permanentf("unsupported campaign channel: %s", campaign.Channel)
	}

	// Inbox campaigns have no external provider dispatch and no sender
	// identity. The rendered template data ({title, body}) is stored directly
	// as the inbox message content, mirroring how a message created from the
	// user inbox tab is shaped.
	if channel == providers.ChannelInbox {
		return []renderedCampaignInboxMessage{{
			Channel:         channel,
			TemplateID:      template.ID,
			Variant:         template.Variant,
			RenderedPayload: template.Data,
		}}, nil
	}

	if channel == providers.ChannelPush {
		pushDispatches, err := resolvePushDispatches(ctx, mgmt, event.ProjectID, event.UserID, usrs)
		if err != nil {
			return nil, fmt.Errorf("resolve push providers: %w", err)
		}

		pushPayload := renderedPushPayload{Dispatches: make([]renderedPushDispatch, 0, len(pushDispatches))}
		for _, dispatch := range pushDispatches {
			var config map[string]any
			if err := json.Unmarshal(dispatch.Provider.Data, &config); err != nil {
				return nil, Permanent(err)
			}

			opts := &channels.ComposeOptions{Devices: dispatch.Devices}
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
				return nil, Permanent(err)
			}

			summary, err := inboxSummaryFromPayload(channel, request.Payload)
			if err != nil {
				return nil, Permanent(err)
			}
			pushPayload.Title = summary.Title
			if summary.Body != nil {
				pushPayload.Body = *summary.Body
			}

			pushPayload.Dispatches = append(pushPayload.Dispatches, renderedPushDispatch{
				ProviderID: dispatch.Provider.ID,
				Payload:    request.Payload,
			})
		}

		payload, err := json.Marshal(pushPayload)
		if err != nil {
			return nil, err
		}

		return []renderedCampaignInboxMessage{{
			Channel:         channel,
			TemplateID:      template.ID,
			Variant:         template.Variant,
			RenderedPayload: payload,
		}}, nil
	}

	activeProvider, templateSender, err := resolveMessageProvider(ctx, mgmt, event.ProjectID, template)
	if err != nil {
		return nil, fmt.Errorf("resolve provider: %w", err)
	}

	var config map[string]any
	if err := json.Unmarshal(activeProvider.Data, &config); err != nil {
		return nil, Permanent(err)
	}

	providerDefaultSender, err := channels.ResolveProviderDefaultFrom(ctx, mgmt.SenderIdentitiesStore, event.ProjectID, config)
	if err != nil {
		return nil, fmt.Errorf("resolve provider default from: %w", err)
	}

	opts := &channels.ComposeOptions{}
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
		return nil, Permanent(err)
	}

	return []renderedCampaignInboxMessage{{
		Channel:          channel,
		SenderIdentityID: template.SenderIdentityID,
		TemplateID:       template.ID,
		Variant:          template.Variant,
		RenderedPayload:  request.Payload,
	}}, nil
}

type inboxSummary struct {
	Title string
	Body  *string
}

func inboxSummaryFromPayload(channel providers.Channel, payload json.RawMessage) (inboxSummary, error) {
	switch channel {
	case providers.ChannelEmail:
		var email providers.EmailPayload
		if err := json.Unmarshal(payload, &email); err != nil {
			return inboxSummary{}, err
		}
		body := email.Text
		if body == "" {
			body = email.HTML
		}
		return inboxSummary{Title: email.Subject, Body: stringPtrOrNil(body)}, nil
	case providers.ChannelSMS:
		var sms providers.SMSPayload
		if err := json.Unmarshal(payload, &sms); err != nil {
			return inboxSummary{}, err
		}
		return inboxSummary{Title: sms.Body, Body: stringPtrOrNil(sms.Body)}, nil
	case providers.ChannelPush:
		var push providers.PushPayload
		if err := json.Unmarshal(payload, &push); err != nil {
			return inboxSummary{}, err
		}
		return inboxSummary{Title: push.Title, Body: stringPtrOrNil(push.Body)}, nil
	default:
		return inboxSummary{}, fmt.Errorf("unsupported channel: %s", channel)
	}
}

func stringPtrOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
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
