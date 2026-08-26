package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	internalProviders "github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/lunogram/platform/pkg/modules"
	providers "github.com/lunogram/platform/pkg/modules/providers"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// InboxHandler holds the shared infrastructure used by all inbox handlers:
// publisher, provider registry, rate limiter, and the database connection for
// transactional message creation. It has no knowledge of subject type (user vs
// organisation); that lives on the concrete handler types that embed it.
type InboxHandler struct {
	logger   *zap.Logger
	db       *sqlx.DB
	mgmt     *management.State
	registry *internalProviders.Registry
	pub      pubsub.Publisher
	limiter  *Limiter
	gate     *complianceGate
}

// newInboxHandler builds the infrastructure shared by the user- and
// organisation-scoped inbox handlers, including the compliance gate every
// outbound message is offered to.
func newInboxHandler(
	logger *zap.Logger,
	db *sqlx.DB,
	mgmt *management.State,
	usrs *subjects.State,
	registry *internalProviders.Registry,
	pub pubsub.Publisher,
	limiter *Limiter,
) InboxHandler {
	return InboxHandler{
		logger:   logger,
		db:       db,
		mgmt:     mgmt,
		registry: registry,
		pub:      pub,
		limiter:  limiter,
		gate: &complianceGate{
			suppressions: mgmt,
			outcomes:     usrs.InboxStore,
			pub:          pub,
			logger:       logger,
		},
	}
}

// PublishInboxLifecycleEvent publishes an inbox lifecycle event (created,
// sent, read, archived) to the event processing pipeline. It routes to
// the correct user or organization events subject based on message ownership.
//
// Events are not deduplicated; each call produces a new event occurrence.
// This allows tracking repeated actions (e.g. multiple reads of the same
// message) as individual events.
func PublishInboxLifecycleEvent(ctx context.Context, pub pubsub.Publisher, message *subjects.InboxMessage, eventName string) error {
	if pub == nil {
		return fmt.Errorf("inbox events: publisher is nil")
	}
	if message == nil {
		return fmt.Errorf("inbox events: message is nil")
	}

	data := map[string]any{"message_id": message.ID.String(), "channel": string(message.Channel)}
	if message.ExternalID != nil {
		data["external_id"] = *message.ExternalID
	}
	if message.CampaignID != nil {
		data["campaign_id"] = message.CampaignID.String()
	}
	if message.BroadcastID != nil {
		data["broadcast_id"] = message.BroadcastID.String()
	}

	switch {
	case message.UserID != nil:
		return pub.Publish(ctx, schemas.UserEventsProcess(message.ProjectID), schemas.UserEvent{
			ProjectID: message.ProjectID,
			UserID:    *message.UserID,
			Name:      eventName,
			Data:      data,
		})
	case message.OrganizationID != nil:
		return pub.Publish(ctx, schemas.OrganizationEventsProcess(message.ProjectID), schemas.OrganizationEvent{
			ProjectID:      message.ProjectID,
			OrganizationID: *message.OrganizationID,
			Name:           eventName,
			Data:           data,
		})
	default:
		return fmt.Errorf("inbox events: message %s has neither user_id nor organization_id", message.ID)
	}
}

// PublishInboxOutcome announces that a message reached a terminal outcome,
// either sent or failed.
//
// It publishes twice on purpose. The tracked lifecycle event goes onto the
// user/organization events subject, where it is stored and countable by rules
// like every other lifecycle transition. The same payload also goes onto the
// inbox settlement subject the broadcast consumer reads, which is what advances
// the broadcast's counters and completes it.
func PublishInboxOutcome(ctx context.Context, pub pubsub.Publisher, message *subjects.InboxMessage, eventName string) error {
	if err := PublishInboxLifecycleEvent(ctx, pub, message, eventName); err != nil {
		return err
	}

	data := map[string]any{"message_id": message.ID.String()}

	switch {
	case message.UserID != nil:
		subject := schemas.UserInboxSent(message.ProjectID)
		if eventName == schemas.EventInboxMessageFailed {
			subject = schemas.UserInboxFailed(message.ProjectID)
		}
		return pub.Publish(ctx, subject, schemas.UserEvent{
			ProjectID: message.ProjectID,
			UserID:    *message.UserID,
			Name:      eventName,
			Data:      data,
		})
	case message.OrganizationID != nil:
		subject := schemas.OrganizationInboxSent(message.ProjectID)
		if eventName == schemas.EventInboxMessageFailed {
			subject = schemas.OrganizationInboxFailed(message.ProjectID)
		}
		return pub.Publish(ctx, subject, schemas.OrganizationEvent{
			ProjectID:      message.ProjectID,
			OrganizationID: *message.OrganizationID,
			Name:           eventName,
			Data:           data,
		})
	default:
		return fmt.Errorf("inbox events: message %s has neither user_id nor organization_id", message.ID)
	}
}

type providerDispatchConfig struct {
	Provider *management.Provider
	Config   map[string]any
}

// LoadProvider fetches the provider record by ID, looks up its WASM module in
// the registry, and decodes the persisted configuration.
func (h *InboxHandler) LoadProvider(ctx context.Context, providerID uuid.UUID) (providerDispatchConfig, *internalProviders.Provider, error) {
	provider, err := h.mgmt.ProvidersStore.GetProvider(ctx, providerID)
	if err != nil {
		return providerDispatchConfig{}, nil, err
	}

	module, exists := h.registry.Get(provider.Module)
	if !exists {
		return providerDispatchConfig{}, nil, Permanentf("module %s not found", provider.Module)
	}

	var config map[string]any
	if err := json.Unmarshal(provider.Data, &config); err != nil {
		return providerDispatchConfig{}, nil, Permanent(err)
	}

	return providerDispatchConfig{Provider: provider, Config: config}, module, nil
}

// LoadProviderFromSenderIdentity resolves the provider associated with a
// sender identity. Used by email/SMS dispatch where the message references a
// sender identity rather than a provider directly.
func (h *InboxHandler) LoadProviderFromSenderIdentity(ctx context.Context, projectID, senderIdentityID uuid.UUID) (*management.SenderIdentity, providerDispatchConfig, *internalProviders.Provider, error) {
	identity, err := h.mgmt.SenderIdentitiesStore.GetSenderIdentity(ctx, projectID, senderIdentityID)
	if err != nil {
		return nil, providerDispatchConfig{}, nil, fmt.Errorf("get sender identity: %w", err)
	}
	cfg, mod, err := h.LoadProvider(ctx, identity.ProviderID)
	return identity, cfg, mod, err
}

func (h *InboxHandler) throttleProvider(ctx context.Context, logger *zap.Logger, module *internalProviders.Provider, provider *management.Provider, msg jetstream.Msg) error {
	rateLimit := providers.ResolveLimit(
		providers.ProviderKey(provider.ID),
		module.Manifest().Spec.RateLimit,
		provider.RateLimit,
		provider.RateInterval,
	)
	return h.limiter.Throttle(ctx, logger, rateLimit, msg)
}

// inboxContent is the generic content format sent by the management UI.
type inboxContent struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// composePayload transforms the raw inbox message content into a channel-
// specific provider payload. When the content is in the generic {title, body}
// format (e.g. from the management UI), it builds a proper EmailPayload or
// SMSPayload using the sender identity and recipient address. If the content
// already contains provider-level fields (e.g. "subject" for email), it is
// passed through as-is.
func composePayload(channel providers.Channel, content json.RawMessage, senderIdentity *management.SenderIdentity, to string) (json.RawMessage, error) {
	// Try to detect the generic inbox format.
	var generic inboxContent
	if err := json.Unmarshal(content, &generic); err != nil {
		return content, nil // not JSON, pass through
	}

	// If the content already has channel-specific fields, pass through.
	var probe map[string]any
	if err := json.Unmarshal(content, &probe); err == nil {
		if _, hasSubject := probe["subject"]; hasSubject {
			return content, nil // already an EmailPayload
		}
		if _, hasFrom := probe["from"]; hasFrom {
			return content, nil // already composed
		}
	}

	// No title means this isn't the generic format — pass through.
	if generic.Title == "" && generic.Body == "" {
		return content, nil
	}

	// The generic format requires a recipient address to compose a provider
	// payload. Organization email/SMS dispatch passes an empty "to" and relies
	// on the content already carrying channel-specific recipient fields (which
	// would have been passed through above). Reaching here with no recipient is
	// a misconfiguration that will never succeed, so fail permanently.
	if to == "" && (channel == providers.ChannelEmail || channel == providers.ChannelSMS) {
		return nil, fmt.Errorf("generic %s content has no recipient address", channel)
	}

	switch channel {
	case providers.ChannelEmail:
		fromAddress := senderIdentity.Address()
		var fromName string
		if traits := senderIdentity.TraitsMap(); traits != nil {
			if name, _ := traits["name"].(string); name != "" {
				fromName = name
			}
		}
		payload := providers.EmailPayload{
			To: to,
			From: providers.EmailAddress{
				Name:    fromName,
				Address: fromAddress,
			},
			Subject: generic.Title,
			HTML:    generic.Body,
			Text:    generic.Body,
		}
		return json.Marshal(payload)

	case providers.ChannelSMS:
		fromAddress := senderIdentity.Address()
		payload := providers.SMSPayload{
			To:   to,
			From: fromAddress,
			Body: generic.Body,
		}
		return json.Marshal(payload)

	default:
		return content, nil
	}
}

func (h *InboxHandler) DispatchDirect(ctx context.Context, msg jetstream.Msg, message *subjects.InboxMessage, to string) error {
	if message.SenderIdentityID == nil {
		return Permanentf("inbox message %s has no sender_identity_id", message.ID)
	}

	senderIdentity, providerConfig, providerModule, err := h.LoadProviderFromSenderIdentity(ctx, message.ProjectID, *message.SenderIdentityID)
	if err != nil {
		return err
	}

	if err := h.throttleProvider(ctx, h.logger, providerModule, providerConfig.Provider, msg); err != nil {
		return err
	}

	payload, err := composePayload(providers.Channel(message.Channel), message.Content, senderIdentity, to)
	if err != nil {
		return Permanent(fmt.Errorf("compose payload for message %s: %w", message.ID, err))
	}

	request := providers.SendRequest[map[string]any]{
		Channel: providers.Channel(message.Channel),
		Config:  providerConfig.Config,
		Payload: payload,
		Metadata: map[string]string{
			providers.MetadataKeyInboxMessageID: message.ID.String(),
		},
	}

	recipient := dispatchRecipient(providers.Channel(message.Channel), payload, to)
	if err := h.gate.Allow(ctx, message, recipient); err != nil {
		return err
	}

	if _, err := providerModule.Send(ctx, request); err != nil {
		h.logger.Error("failed to send inbox message via provider", zap.Error(err), zap.Stringer("message_id", message.ID))
		return h.gate.SendFailed(ctx, message, recipient, err)
	}

	return nil
}

// FanOutPush publishes one NATS message per (inbox_message, provider) pair on
// the dispatch subject. The Msg-Id is
// "inbox-dispatch:<inbox_message_id>:<provider_id>" so JetStream dedupes any
// re-fan-out caused by a redelivered inbox.process message.
func (h *InboxHandler) FanOutPush(ctx context.Context, pushSubject func(uuid.UUID) schemas.Subject, message *subjects.InboxMessage) error {
	var rendered renderedPushPayload
	if err := json.Unmarshal(message.Content, &rendered); err != nil {
		return Permanent(fmt.Errorf("unmarshal push payload for message %s: %w", message.ID, err))
	}
	if len(rendered.Dispatches) == 0 {
		return Permanentf("push inbox message %s has no provider dispatches", message.ID)
	}
	if h.pub == nil {
		return Permanentf("push inbox dispatch requires a publisher (message %s)", message.ID)
	}

	subject := pushSubject(message.ProjectID)
	if subject == "" {
		return Permanentf("push inbox dispatch: empty subject for message %s", message.ID)
	}

	scope := "user"
	if message.OrganizationID != nil {
		scope = "organization"
	}

	for _, dispatch := range rendered.Dispatches {
		event := schemas.InboxPushDispatch{
			ProjectID:      message.ProjectID,
			InboxMessageID: message.ID,
			ProviderID:     dispatch.ProviderID,
			Scope:          scope,
			Payload:        dispatch.Payload,
		}
		msgID := schemas.InboxDispatchMsgID(message.ID, dispatch.ProviderID)
		if err := h.pub.Publish(ctx, subject, event, pubsub.WithMsgID(msgID)); err != nil {
			return fmt.Errorf("publish push inbox dispatch for provider %s: %w", dispatch.ProviderID, err)
		}
	}

	return nil
}

// inboxMessageParams projects the wire-format inbox payload onto the store
// parameter struct.
func inboxMessageParams(inbox schemas.InboxMessage) subjects.InboxMessageParams {
	return subjects.InboxMessageParams{
		ExternalID:       inbox.ExternalID,
		Channel:          modules.Channel(inbox.Channel),
		SenderIdentityID: inbox.SenderIdentityID,
		CampaignID:       inbox.CampaignID,
		BroadcastID:      inbox.BroadcastID,
		Content:          inbox.Content,
		Data:             inbox.Data,
		Tags:             inbox.Tags,
		Priority:         inbox.Priority,
		Source:           inbox.Source,
		ScheduledAt:      inbox.ScheduledAt,
		ExpiresAt:        inbox.ExpiresAt,
	}
}
