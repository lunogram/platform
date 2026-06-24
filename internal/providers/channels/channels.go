package channels

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/providers/linkwrap"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/lunogram/platform/pkg/modules/providers"
	"go.uber.org/zap"
)

// Provider data keys for default from configuration.
const (
	ProviderKeyDefaultFrom = "default_from"
)

// LinkWrapConfig holds the parameters needed to apply click-tracking link
// wrapping to outgoing messages. When a non-nil *LinkWrapConfig is passed to
// the Compose/ComposePayload functions, links in the message body are rewritten
// with encrypted tracking URLs.
type LinkWrapConfig struct {
	Key         []byte
	TrackingURL string
	ProjectID   uuid.UUID
	CampaignID  uuid.UUID
	UserID      uuid.UUID
}

// ResolveProviderDefaultFrom resolves the provider's default_from config value.
// If the value is a sender identity UUID, it looks up the identity via the store.
// If it's a plain address string, it wraps it in a SenderIdentity with the
// address stored in traits. Returns nil if no default_from is configured.
func ResolveProviderDefaultFrom(ctx context.Context, store *management.SenderIdentitiesStore, projectID uuid.UUID, config map[string]any) (*management.SenderIdentity, error) {
	defaultFrom, _ := config[ProviderKeyDefaultFrom].(string)
	if defaultFrom == "" {
		return nil, nil
	}

	id, err := uuid.Parse(defaultFrom)
	if err != nil {
		// Not a UUID — wrap the plain address string as a SenderIdentity.
		traits, _ := json.Marshal(map[string]any{"address": defaultFrom})
		return &management.SenderIdentity{Traits: traits}, nil
	}

	identity, err := store.GetSenderIdentity(ctx, projectID, id)
	if err != nil {
		return nil, fmt.Errorf("resolve provider default_from identity %s: %w", id, err)
	}

	return identity, nil
}

type ComposeOptions struct {
	Devices  subjects.Devices
	LinkWrap *LinkWrapConfig
}

// wrapLinks applies link wrapping to template data based on the channel type.
// If lw is nil the data is returned unchanged.
func wrapLinks(logger *zap.Logger, channel providers.Channel, data json.RawMessage, config *LinkWrapConfig) json.RawMessage {
	if config == nil {
		return data
	}

	switch channel {
	case providers.ChannelEmail:
		wrapped, err := linkwrap.WrapEmailLinks(data, config.Key, config.TrackingURL, config.ProjectID, config.CampaignID, config.UserID)
		if err != nil {
			logger.Warn("link wrapping failed for email, sending with original URLs", zap.Error(err))
			return data
		}
		return wrapped
	case providers.ChannelPush:
		wrapped, err := linkwrap.WrapPushLinks(data, config.Key, config.TrackingURL, config.ProjectID, config.CampaignID, config.UserID)
		if err != nil {
			logger.Warn("link wrapping failed for push, sending with original URLs", zap.Error(err))
			return data
		}
		return wrapped
	}

	return data
}

// ComposePayload creates a SendRequest for the given channel using raw template
// data and an explicit recipient address (email address, phone number, etc.).
// Both templateSender and providerDefaultSender should be pre-resolved (or nil).
// When lw is non-nil, links in the message body are rewritten for click tracking.
func ComposePayload(ctx context.Context, logger *zap.Logger, channel string, templateSender, providerDefaultSender *management.SenderIdentity, config map[string]any, templateData json.RawMessage, to string, lw *LinkWrapConfig) (providers.SendRequest[map[string]any], error) {
	templateData = wrapLinks(logger, providers.Channel(channel), templateData, lw)

	switch providers.Channel(channel) {
	case providers.ChannelEmail:
		return ComposeEmailPayload(ctx, templateSender, providerDefaultSender, config, templateData, to)
	case providers.ChannelSMS:
		return ComposeSMSPayload(ctx, templateSender, providerDefaultSender, config, templateData, to)
	case providers.ChannelPush:
		return providers.SendRequest[map[string]any]{}, fmt.Errorf("push channel does not support composing by recipient address")
	default:
		return providers.SendRequest[map[string]any]{}, fmt.Errorf("unsupported channel: %s", channel)
	}
}

// Compose creates a SendRequest for the given channel and user.
// Both templateSender and providerDefaultSender should be pre-resolved (or nil).
func Compose(ctx context.Context, logger *zap.Logger, channel providers.Channel, templateSender, providerDefaultSender *management.SenderIdentity, config map[string]any, template management.Template, user *subjects.User, opts *ComposeOptions) (providers.SendRequest[map[string]any], error) {
	var lw *LinkWrapConfig
	if opts != nil {
		lw = opts.LinkWrap
	}

	template.Data = wrapLinks(logger, channel, template.Data, lw)

	switch channel {
	case providers.ChannelEmail:
		return ComposeEmail(ctx, templateSender, providerDefaultSender, config, template, user)
	case providers.ChannelSMS:
		return ComposeSMS(ctx, templateSender, providerDefaultSender, config, template, user)
	case providers.ChannelPush:
		if opts == nil {
			return providers.SendRequest[map[string]any]{}, fmt.Errorf("push channel requires devices")
		}
		return ComposePush(ctx, config, template, user, opts.Devices)
	default:
		return providers.SendRequest[map[string]any]{}, fmt.Errorf("unsupported channel: %s", channel)
	}
}
