package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	internalProviders "github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	wasmProviders "github.com/lunogram/platform/internal/wasm/providers"
	"github.com/lunogram/platform/pkg/modules"
	providers "github.com/lunogram/platform/pkg/modules/providers"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// OrganizationInboxHandler handles all inbox consumers scoped to
// organisations. It embeds InboxHandler for shared provider/dispatch logic and
// adds the subjects.State needed for organisation-specific store calls.
type OrganizationInboxHandler struct {
	InboxHandler
	usrs *subjects.State
}

// NewOrganizationInboxHandler constructs an OrganizationInboxHandler. All
// parameters are required.
func NewOrganizationInboxHandler(
	logger *zap.Logger,
	db *sqlx.DB,
	mgmt *management.State,
	usrs *subjects.State,
	registry *internalProviders.Registry,
	pub pubsub.Publisher,
	limiter *Limiter,
) *OrganizationInboxHandler {
	return &OrganizationInboxHandler{
		InboxHandler: InboxHandler{
			logger:   logger,
			db:       db,
			mgmt:     mgmt,
			registry: registry,
			pub:      pub,
			limiter:  limiter,
		},
		usrs: usrs,
	}
}

// Messages handles organisation-scoped inbox messages. The inbox row is
// created in a transaction and the inbox.message.created event is published
// only after that transaction commits.
func (h *OrganizationInboxHandler) Messages() HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		var inbox schemas.InboxMessage
		if err := json.Unmarshal(msg.Data(), &inbox); err != nil {
			h.logger.Error("failed to unmarshal organization inbox message", zap.Error(err))
			return Permanent(err)
		}
		if inbox.Channel == "" {
			inbox.Channel = string(modules.ChannelInbox)
		}

		var created *subjects.InboxMessage
		var err error

		// When a MessageID is provided (e.g. rate-limited retry), load
		// the existing message instead of creating a new one.
		if inbox.MessageID != uuid.Nil {
			created, err = h.usrs.InboxStore.GetOrganizationInboxMessageByID(ctx, inbox.MessageID)
			if errors.Is(err, sql.ErrNoRows) {
				h.logger.Error("inbox message not found", zap.Stringer("message_id", inbox.MessageID))
				return Permanent(err)
			}
			if err != nil {
				h.logger.Error("failed to load existing inbox message", zap.Error(err), zap.Stringer("message_id", inbox.MessageID))
				return err
			}
		}

		if created == nil {
			inbox.SubjectID, err = h.resolveOrganizationID(ctx, inbox.ProjectID, inbox.SubjectID, inbox.Identifiers)
			if err != nil {
				return err
			}
		}

		if created == nil {
			created, err = h.createAndPublish(ctx, inbox)
			if errors.Is(err, sql.ErrNoRows) {
				h.logger.Info("organization inbox message already exists")
				return nil
			}
			if err != nil {
				h.logger.Error("failed to create organization inbox message", zap.Error(err))
				return err
			}
		}

		// A message scheduled for the future is persisted but not dispatched
		// here. The cluster scheduler re-injects it onto this subject once
		// scheduled_at has passed, at which point it flows through to dispatch.
		if !created.IsDue() {
			h.logger.Debug("organization inbox message not yet due, awaiting scheduler", zap.Stringer("message_id", created.ID), zap.Time("scheduled_at", created.ScheduledAt))
			return nil
		}

		if created.Channel != modules.ChannelInbox {
			switch providers.Channel(created.Channel) {
			case providers.ChannelEmail, providers.ChannelSMS:
				// Organizations don't have a top-level email/phone field;
				// the content must already contain the recipient address.
				if err := h.DispatchDirect(ctx, msg, created, ""); err != nil {
					if limit, ok := IsRateLimited(err); ok {
						inbox.MessageID = created.ID
						if pubErr := h.pub.Publish(ctx, schemas.OrganizationInboxProcess(inbox.ProjectID), inbox, pubsub.At(time.Now().Add(limit.RetryAfter))); pubErr != nil {
							return fmt.Errorf("schedule rate-limited organization inbox message: %w", pubErr)
						}
						return nil
					}
					return err
				}
			case providers.ChannelPush:
				return h.FanOutPush(ctx, schemas.OrganizationInboxDispatch, created)
			default:
				return Permanentf("unsupported inbox channel: %s", created.Channel)
			}
		}

		marked, err := h.usrs.InboxStore.MarkOrganizationInboxMessageSent(ctx, created.ID)
		if err != nil {
			h.logger.Error("failed to mark inbox message sent", zap.Error(err), zap.Stringer("message_id", created.ID))
			return err
		}
		if marked {
			if err := PublishInboxOutcome(ctx, h.pub, created, schemas.EventInboxMessageSent); err != nil {
				h.logger.Error("failed to publish inbox sent event", zap.Error(err), zap.Stringer("message_id", created.ID))
			}
		}

		return nil
	}
}

// Read consumes the organizations.inbox.read.<projectID> subject.
func (h *OrganizationInboxHandler) Read() HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		var event schemas.InboxStateEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			h.logger.Error("failed to unmarshal organization inbox read event", zap.Error(err))
			return Permanent(err)
		}

		var err error
		event.SubjectID, err = h.resolveOrganizationID(ctx, event.ProjectID, event.SubjectID, event.Identifiers)
		if err != nil {
			return err
		}

		message, transitioned, err := h.usrs.ReadOrganizationInboxMessage(ctx, event.ProjectID, event.SubjectID, event.MessageID)
		if errors.Is(err, sql.ErrNoRows) {
			h.logger.Error("inbox message not found for open", zap.Stringer("message_id", event.MessageID))
			return Permanent(err)
		}
		if err != nil {
			h.logger.Error("failed to read organization inbox message", zap.Error(err))
			return err
		}
		if !transitioned {
			h.logger.Info("organization inbox message already read", zap.Stringer("message_id", message.ID))
			return nil
		}

		if err := PublishInboxLifecycleEvent(ctx, h.pub, message, schemas.EventInboxMessageRead); err != nil {
			h.logger.Error("failed to publish organization inbox read event", zap.Error(err))
			return err
		}
		return nil
	}
}

// Archived consumes the organizations.inbox.archived.<projectID> subject.
func (h *OrganizationInboxHandler) Archived() HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		var event schemas.InboxStateEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			h.logger.Error("failed to unmarshal organization inbox archived event", zap.Error(err))
			return Permanent(err)
		}

		var err error
		event.SubjectID, err = h.resolveOrganizationID(ctx, event.ProjectID, event.SubjectID, event.Identifiers)
		if err != nil {
			return err
		}

		message, transitioned, err := h.usrs.ArchiveOrganizationInboxMessage(ctx, event.ProjectID, event.SubjectID, event.MessageID)
		if errors.Is(err, sql.ErrNoRows) {
			h.logger.Error("inbox message not found for archive", zap.Stringer("message_id", event.MessageID))
			return Permanent(err)
		}
		if err != nil {
			h.logger.Error("failed to archive organization inbox message", zap.Error(err))
			return err
		}
		if !transitioned {
			h.logger.Info("organization inbox message already archived", zap.Stringer("message_id", message.ID))
			return nil
		}

		if err := PublishInboxLifecycleEvent(ctx, h.pub, message, schemas.EventInboxMessageArchived); err != nil {
			h.logger.Error("failed to publish organization inbox archived event", zap.Error(err))
			return err
		}
		return nil
	}
}

// Dispatch handles per-provider push fan-out for organisation-scoped inbox
// messages.
func (h *OrganizationInboxHandler) Dispatch() HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		var event schemas.InboxPushDispatch
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			h.logger.Error("failed to unmarshal push inbox dispatch", zap.Error(err))
			return Permanent(err)
		}

		logger := h.logger.With(
			zap.Stringer("project_id", event.ProjectID),
			zap.Stringer("inbox_message_id", event.InboxMessageID),
			zap.Stringer("provider_id", event.ProviderID),
		)

		providerConfig, providerModule, err := h.LoadProvider(ctx, event.ProviderID)
		if err != nil {
			logger.Error("failed to load provider for push dispatch", zap.Error(err))
			return err
		}

		if err := h.throttleProvider(ctx, logger, providerModule, providerConfig.Provider, msg); err != nil {
			if limit, ok := IsRateLimited(err); ok {
				subject := schemas.OrganizationInboxDispatch(event.ProjectID)
				if pubErr := h.pub.Publish(ctx, subject, event, pubsub.At(time.Now().Add(limit.RetryAfter))); pubErr != nil {
					return fmt.Errorf("schedule rate-limited push dispatch: %w", pubErr)
				}
				return nil
			}
			return err
		}

		request := providers.SendRequest[map[string]any]{
			Channel: providers.ChannelPush,
			Config:  providerConfig.Config,
			Payload: event.Payload,
			Metadata: map[string]string{
				providers.MetadataKeyInboxMessageID: event.InboxMessageID.String(),
			},
		}

		if _, err := providerModule.Send(ctx, request); err != nil {
			logger.Error("failed to send push inbox dispatch via provider", zap.Error(err))
			var providerErr *wasmProviders.ProviderError
			if errors.As(err, &providerErr) && providerErr.IsPermanent() {
				return Permanent(err)
			}
			return err
		}

		marked, err := h.usrs.InboxStore.MarkOrganizationInboxMessageSent(ctx, event.InboxMessageID)
		if err != nil {
			logger.Error("failed to mark inbox message sent", zap.Error(err), zap.Stringer("message_id", event.InboxMessageID))
			return err
		}
		if marked {
			message, err := h.usrs.InboxStore.GetOrganizationInboxMessageByID(ctx, event.InboxMessageID)
			if err != nil {
				logger.Error("failed to load inbox message for sent event", zap.Error(err))
			} else {
				if pubErr := PublishInboxOutcome(ctx, h.pub, message, schemas.EventInboxMessageSent); pubErr != nil {
					logger.Error("failed to publish inbox sent event", zap.Error(pubErr))
				}
			}
		}

		return nil
	}
}

// Sent reacts to inbox.message.sent events for organisation-scoped messages.
// When the sent message belongs to a broadcast, it delegates to
// completeBroadcastIfDone to atomically increment the counter and transition
// state when all messages have been dispatched.
func (h *OrganizationInboxHandler) Sent() HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		messageID, err := parseSentMessageID(msg)
		if err != nil {
			h.logger.Error("invalid inbox sent event", zap.Error(err))
			return Permanent(err)
		}

		message, err := h.usrs.InboxStore.GetOrganizationInboxMessageByID(ctx, messageID)
		if errors.Is(err, sql.ErrNoRows) {
			h.logger.Error("inbox message not found for sent handler", zap.Stringer("message_id", messageID))
			return Permanent(err)
		}
		if err != nil {
			h.logger.Error("failed to load inbox message for sent handler", zap.Error(err), zap.Stringer("message_id", messageID))
			return err
		}

		return h.completeBroadcastIfDone(ctx, message)
	}
}

// resolveOrganizationID returns subjectID unchanged when already set,
// otherwise performs an identifier lookup and returns the resolved
// organisation ID.
func (h *OrganizationInboxHandler) resolveOrganizationID(ctx context.Context, projectID, subjectID uuid.UUID, identifiers []subjects.ExternalIDParam) (uuid.UUID, error) {
	if subjectID != uuid.Nil {
		return subjectID, nil
	}
	if len(identifiers) == 0 {
		h.logger.Error("organization inbox message requires subject_id or identifiers")
		return uuid.Nil, Permanent(fmt.Errorf("organization inbox message requires subject_id or identifiers"))
	}
	organizationID, err := h.usrs.LookupOrganizationID(ctx, projectID, identifiers)
	if errors.Is(err, subjects.ErrOrgNotFound) || errors.Is(err, subjects.ErrConflictingIdentifiers) {
		h.logger.Error("failed to lookup organization for inbox message", zap.Error(err))
		return uuid.Nil, Permanent(err)
	}
	if err != nil {
		h.logger.Error("failed to lookup organization for inbox message", zap.Error(err))
		return uuid.Nil, err
	}
	return organizationID, nil
}

// createAndPublish inserts the organisation inbox row inside a transaction
// and, after that transaction commits, publishes the inbox.message.created
// event when the message is due.
func (h *OrganizationInboxHandler) createAndPublish(ctx context.Context, inbox schemas.InboxMessage) (created *subjects.InboxMessage, err error) {
	tx, err := h.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
				h.logger.Error("failed to rollback tx", zap.Error(rbErr))
			}
		}
	}()

	txStore := subjects.NewInboxStore(tx)
	created, err = txStore.CreateOrganizationInboxMessage(ctx, inbox.ProjectID, inbox.SubjectID, inboxMessageParams(inbox))
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	if created.IsDue() {
		if pubErr := PublishInboxLifecycleEvent(ctx, h.pub, created, schemas.EventInboxMessageCreated); pubErr != nil {
			h.logger.Error("failed to publish inbox created event", zap.Error(pubErr), zap.Stringer("message_id", created.ID))
		}
	}
	return created, nil
}
