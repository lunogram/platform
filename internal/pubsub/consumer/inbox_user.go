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

// UserInboxHandler handles all inbox consumers scoped to users.
// It embeds InboxHandler for shared provider/dispatch/rate-limit logic and
// adds the subjects.State needed for user-specific store calls.
type UserInboxHandler struct {
	InboxHandler
	usrs *subjects.State
}

// NewUserInboxHandler constructs a UserInboxHandler. All parameters are
// required.
func NewUserInboxHandler(
	logger *zap.Logger,
	db *sqlx.DB,
	mgmt *management.State,
	usrs *subjects.State,
	registry *internalProviders.Registry,
	pub pubsub.Publisher,
	limiter *Limiter,
) *UserInboxHandler {
	return &UserInboxHandler{
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

// Messages ingests inbox messages published by the API for a specific user.
// It creates the inbox row and, when due, publishes the inbox.message.created
// event to the bus. Idempotency across retries is provided by the row's
// external_id unique constraint and by JetStream Msg-Id dedup on the published
// event. It then dispatches via the message's provider (if any), marks
// sent_at, and publishes the inbox.message.sent lifecycle event. When called
// with a MessageID (from the campaign handler), it skips row creation and
// dispatches the existing message.
func (h *UserInboxHandler) Messages() HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		var inbox schemas.InboxMessage
		if err := json.Unmarshal(msg.Data(), &inbox); err != nil {
			h.logger.Error("failed to unmarshal user inbox message", zap.Error(err))
			return Permanent(err)
		}
		if inbox.Channel == "" {
			inbox.Channel = string(modules.ChannelInbox)
		}

		var created *subjects.InboxMessage
		var err error

		// When a MessageID is provided (from campaign handler), load the existing message.
		if inbox.MessageID != uuid.Nil {
			created, err = h.usrs.InboxStore.GetUserInboxMessageByID(ctx, inbox.MessageID)
			if errors.Is(err, sql.ErrNoRows) {
				h.logger.Error("inbox message not found", zap.Stringer("message_id", inbox.MessageID))
				return Permanent(err)
			}
			if err != nil {
				h.logger.Error("failed to load existing inbox message", zap.Error(err), zap.Stringer("message_id", inbox.MessageID))
				return err
			}
		}

		// Resolve the subject via identifiers when no SubjectID is set.
		if created == nil {
			inbox.SubjectID, err = h.resolveUserID(ctx, inbox.ProjectID, inbox.SubjectID, inbox.Identifiers)
			if err != nil {
				return err
			}
		}

		// Create the inbox message if it wasn't loaded by MessageID.
		if created == nil {
			created, err = h.createAndPublish(ctx, inbox)
			if errors.Is(err, sql.ErrNoRows) {
				h.logger.Info("user inbox message already exists")
				return nil
			}
			if err != nil {
				h.logger.Error("failed to create user inbox message", zap.Error(err))
				return err
			}
		}

		// A message scheduled for the future is persisted but not dispatched
		// here. The cluster scheduler re-injects it onto this subject once
		// scheduled_at has passed, at which point it flows through to dispatch.
		// This keeps scheduling honored regardless of which path published the
		// message (client/campaign publish immediately; management withholds
		// until due).
		if !created.IsDue() {
			h.logger.Debug("user inbox message not yet due, awaiting scheduler", zap.Stringer("message_id", created.ID), zap.Time("scheduled_at", created.ScheduledAt))
			return nil
		}

		// Dispatch non-inbox channels before marking the message as sent.
		if created.Channel != modules.ChannelInbox {
			switch providers.Channel(created.Channel) {
			case providers.ChannelEmail, providers.ChannelSMS:
				to, err := h.resolveUserRecipient(ctx, created)
				if err != nil {
					return err
				}
				if err := h.DispatchDirect(ctx, msg, created, to); err != nil {
					if limit, ok := IsRateLimited(err); ok {
						inbox.MessageID = created.ID
						if pubErr := h.pub.Publish(ctx, schemas.UserInboxProcess(inbox.ProjectID), inbox, pubsub.At(time.Now().Add(limit.RetryAfter))); pubErr != nil {
							return fmt.Errorf("schedule rate-limited user inbox message: %w", pubErr)
						}
						return nil
					}
					return err
				}
			case providers.ChannelPush:
				return h.FanOutPush(ctx, schemas.UserInboxDispatch, created)
			default:
				return Permanentf("unsupported inbox channel: %s", created.Channel)
			}
		}

		// Mark the message as sent. The idempotent guard (sent_at IS NULL)
		// ensures we only publish the lifecycle event once across retries.
		marked, err := h.usrs.InboxStore.MarkUserInboxMessageSent(ctx, created.ID)
		if err != nil {
			h.logger.Error("failed to mark inbox message sent", zap.Error(err), zap.Stringer("message_id", created.ID))
			return err
		}
		if marked {
			if err := PublishInboxOutcome(ctx, h.pub, created, schemas.EventInboxMessageSent); err != nil {
				h.logger.Error("failed to publish inbox sent event", zap.Error(err), zap.Stringer("message_id", created.ID))
				// Non-fatal: sent_at is persisted, event can be replayed.
			}
		}

		return nil
	}
}

// Read consumes the users.inbox.read.<projectID> subject. It resolves
// the subject, applies the read transition, and publishes the corresponding
// lifecycle event.
func (h *UserInboxHandler) Read() HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		var event schemas.InboxStateEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			h.logger.Error("failed to unmarshal user inbox read event", zap.Error(err))
			return Permanent(err)
		}

		var err error
		event.SubjectID, err = h.resolveUserID(ctx, event.ProjectID, event.SubjectID, event.Identifiers)
		if err != nil {
			return err
		}

		message, transitioned, err := h.usrs.ReadUserInboxMessage(ctx, event.ProjectID, event.SubjectID, event.MessageID)
		if errors.Is(err, sql.ErrNoRows) {
			h.logger.Error("inbox message not found for open", zap.Stringer("message_id", event.MessageID))
			return Permanent(err)
		}
		if err != nil {
			h.logger.Error("failed to read user inbox message", zap.Error(err))
			return err
		}
		if !transitioned {
			h.logger.Info("user inbox message already read", zap.Stringer("message_id", message.ID))
			return nil
		}

		if err := PublishInboxLifecycleEvent(ctx, h.pub, message, schemas.EventInboxMessageRead); err != nil {
			h.logger.Error("failed to publish user inbox read event", zap.Error(err))
			return err
		}
		return nil
	}
}

// Archived consumes the users.inbox.archived.<projectID> subject.
func (h *UserInboxHandler) Archived() HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		var event schemas.InboxStateEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			h.logger.Error("failed to unmarshal user inbox archived event", zap.Error(err))
			return Permanent(err)
		}

		var err error
		event.SubjectID, err = h.resolveUserID(ctx, event.ProjectID, event.SubjectID, event.Identifiers)
		if err != nil {
			return err
		}

		message, transitioned, err := h.usrs.ArchiveUserInboxMessage(ctx, event.ProjectID, event.SubjectID, event.MessageID)
		if errors.Is(err, sql.ErrNoRows) {
			h.logger.Error("inbox message not found for archive", zap.Stringer("message_id", event.MessageID))
			return Permanent(err)
		}
		if err != nil {
			h.logger.Error("failed to archive user inbox message", zap.Error(err))
			return err
		}
		if !transitioned {
			h.logger.Info("user inbox message already archived", zap.Stringer("message_id", message.ID))
			return nil
		}

		if err := PublishInboxLifecycleEvent(ctx, h.pub, message, schemas.EventInboxMessageArchived); err != nil {
			h.logger.Error("failed to publish user inbox archived event", zap.Error(err))
			return err
		}
		return nil
	}
}

// Dispatch handles per-provider push fan-out for user-scoped inbox messages.
// Each dispatch represents a single (inbox_message, provider) pair. JetStream
// Msg-Id deduplication ensures redeliveries do not republish dispatches that
// already landed.
func (h *UserInboxHandler) Dispatch() HandlerFunc {
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
				subject := schemas.UserInboxDispatch(event.ProjectID)
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

		// Mark the inbox message as sent and publish the lifecycle event.
		// The idempotent guard (sent_at IS NULL) ensures we fire only once
		// even if multiple push providers succeed for the same message.
		marked, err := h.usrs.InboxStore.MarkUserInboxMessageSent(ctx, event.InboxMessageID)
		if err != nil {
			logger.Error("failed to mark inbox message sent", zap.Error(err), zap.Stringer("message_id", event.InboxMessageID))
			return err
		}
		if marked {
			message, err := h.usrs.InboxStore.GetUserInboxMessageByID(ctx, event.InboxMessageID)
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

// Sent reacts to inbox.message.sent events for user-scoped messages. When the
// sent message belongs to a broadcast, it delegates to completeBroadcastIfDone
// to atomically increment the counter and transition state when all messages
// have been dispatched.
func (h *UserInboxHandler) Sent() HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		messageID, err := parseSentMessageID(msg)
		if err != nil {
			h.logger.Error("invalid inbox sent event", zap.Error(err))
			return Permanent(err)
		}

		message, err := h.usrs.InboxStore.GetUserInboxMessageByID(ctx, messageID)
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

// resolveUserID returns subjectID unchanged when already set, otherwise
// performs an identifier lookup and returns the resolved user ID.
func (h *UserInboxHandler) resolveUserID(ctx context.Context, projectID, subjectID uuid.UUID, identifiers []subjects.ExternalIDParam) (uuid.UUID, error) {
	if subjectID != uuid.Nil {
		return subjectID, nil
	}
	if len(identifiers) == 0 {
		h.logger.Error("user inbox message requires subject_id or identifiers")
		return uuid.Nil, Permanent(fmt.Errorf("user inbox message requires subject_id or identifiers"))
	}
	userID, err := h.usrs.LookupUserID(ctx, projectID, identifiers)
	if errors.Is(err, subjects.ErrUserNotFound) || errors.Is(err, subjects.ErrConflictingIdentifiers) {
		h.logger.Error("failed to lookup user for inbox message", zap.Error(err))
		return uuid.Nil, Permanent(err)
	}
	if err != nil {
		h.logger.Error("failed to lookup user for inbox message", zap.Error(err))
		return uuid.Nil, err
	}
	return userID, nil
}

// createAndPublish inserts the user inbox row inside a transaction and, after
// the transaction commits, publishes the inbox.message.created event when the
// message is due. Idempotency on retry is provided by the row's external_id
// unique constraint and by JetStream Msg-Id dedup on the published event.
func (h *UserInboxHandler) createAndPublish(ctx context.Context, inbox schemas.InboxMessage) (created *subjects.InboxMessage, err error) {
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
	created, err = txStore.CreateUserInboxMessage(ctx, inbox.ProjectID, inbox.SubjectID, inboxMessageParams(inbox))
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

// resolveUserRecipient looks up the user's email or phone based on the message
// channel to provide the "to" address for provider dispatch.
func (h *UserInboxHandler) resolveUserRecipient(ctx context.Context, message *subjects.InboxMessage) (string, error) {
	if message.UserID == nil {
		return "", Permanentf("inbox message %s has no user_id", message.ID)
	}

	user, err := h.usrs.UsersStore.GetUser(ctx, message.ProjectID, *message.UserID)
	if err != nil {
		return "", fmt.Errorf("get user for inbox dispatch: %w", err)
	}

	switch providers.Channel(message.Channel) {
	case providers.ChannelEmail:
		if user.Email == nil || *user.Email == "" {
			return "", Permanentf("user %s has no email address for email dispatch", user.ID)
		}
		return *user.Email, nil
	case providers.ChannelSMS:
		if user.Phone == nil || *user.Phone == "" {
			return "", Permanentf("user %s has no phone number for SMS dispatch", user.ID)
		}
		return *user.Phone, nil
	default:
		return "", Permanentf("unsupported channel %s for recipient resolution", message.Channel)
	}
}
