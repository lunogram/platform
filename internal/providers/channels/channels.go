package channels

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/lunogram/platform/pkg/modules/providers"
)

// Provider data keys for default from configuration.
const (
	ProviderKeyDefaultFrom = "default_from"
)

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
	Devices subjects.Devices
}

// ComposePayload creates a SendRequest for the given channel using raw template
// data and an explicit recipient address (email address, phone number, etc.).
// Both templateSender and providerDefaultSender should be pre-resolved (or nil).
func ComposePayload(ctx context.Context, channel string, templateSender, providerDefaultSender *management.SenderIdentity, config map[string]any, templateData json.RawMessage, to string) (*providers.SendRequest[map[string]any], error) {
	switch providers.Channel(channel) {
	case providers.ChannelEmail:
		return ComposeEmailPayload(ctx, templateSender, providerDefaultSender, config, templateData, to)
	case providers.ChannelSMS:
		return ComposeSMSPayload(ctx, templateSender, providerDefaultSender, config, templateData, to)
	case providers.ChannelPush:
		return nil, fmt.Errorf("push channel does not support composing by recipient address")
	default:
		return nil, fmt.Errorf("unsupported channel: %s", channel)
	}
}

// Compose creates a SendRequest for the given channel and user.
// Both templateSender and providerDefaultSender should be pre-resolved (or nil).
func Compose(ctx context.Context, channel providers.Channel, templateSender, providerDefaultSender *management.SenderIdentity, config map[string]any, template management.Template, user *subjects.User, opts *ComposeOptions) (*providers.SendRequest[map[string]any], error) {
	switch channel {
	case providers.ChannelEmail:
		return ComposeEmail(ctx, templateSender, providerDefaultSender, config, template, user)
	case providers.ChannelSMS:
		return ComposeSMS(ctx, templateSender, providerDefaultSender, config, template, user)
	case providers.ChannelPush:
		if opts == nil {
			return nil, fmt.Errorf("push channel requires devices")
		}
		return ComposePush(ctx, config, template, user, opts.Devices)
	default:
		return nil, fmt.Errorf("unsupported channel: %s", channel)
	}
}
