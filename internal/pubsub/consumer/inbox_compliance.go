package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/compliance"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	wasmProviders "github.com/lunogram/platform/internal/wasm/providers"
	"github.com/lunogram/platform/pkg/modules"
	providers "github.com/lunogram/platform/pkg/modules/providers"
	"go.uber.org/zap"
)

// suppressionStore is the part of the management suppression store the send
// path uses. Narrowing it here keeps the gate decidable without a database.
type suppressionStore interface {
	IsSuppressed(ctx context.Context, projectID uuid.UUID, recipientPhone string) (bool, error)
	RecordOptOut(ctx context.Context, in management.SuppressionInput) error
}

// inboxOutcomeStore settles an inbox message that will never be delivered.
type inboxOutcomeStore interface {
	MarkUserInboxMessageFailed(ctx context.Context, id uuid.UUID, reason string) (bool, error)
	MarkOrganizationInboxMessageFailed(ctx context.Context, id uuid.UUID, reason string) (bool, error)
}

// errRecipientOptedOut is the cause recorded when the gate itself refuses a
// send, as opposed to a provider reporting the opt-out back to us.
var errRecipientOptedOut = errors.New("recipient has opted out of SMS from this project")

// complianceGate is the send-path chokepoint for SMS consent. Every message
// crossing DispatchDirect is offered to Allow first, and any message that will
// never be delivered - refused here or refused by the provider - is settled
// through Fail so that failed_at, the terminal lifecycle event, and the
// broadcast counters always move together.
//
// The gate deliberately ignores a campaign's transactional flag. Opt-out is a
// property of the recipient's consent, not of the sender's intent, so the only
// bypass is subjects.InboxClassCompliance: an opt-out confirmation and a HELP
// reply must still reach someone who has just opted out.
type complianceGate struct {
	suppressions suppressionStore
	outcomes     inboxOutcomeStore
	pub          pubsub.Publisher
	logger       *zap.Logger
}

// Allow reports whether the message may be handed to its provider. A nil error
// means send; any other return means the message must not be sent, and a
// message refused on consent grounds has already been settled by the time Allow
// returns. A permanent error stops redelivery; a transient one asks for it,
// because an unreadable suppression state must never be read as consent.
func (g *complianceGate) Allow(ctx context.Context, message *subjects.InboxMessage, to string) error {
	if providers.Channel(message.Channel) != providers.ChannelSMS {
		return nil
	}

	if message.Class == subjects.InboxClassCompliance {
		return nil
	}

	suppressed, err := g.suppressions.IsSuppressed(ctx, message.ProjectID, to)
	switch {
	case errors.Is(err, management.ErrInvalidPhoneNumber):
		return g.Fail(ctx, message, providers.ReasonInvalidNumber, err)
	case err != nil:
		return fmt.Errorf("suppression lookup for inbox message %s: %w", message.ID, err)
	case !suppressed:
		return nil
	}

	g.logger.Info("suppressing SMS to opted-out recipient",
		zap.Stringer("project_id", message.ProjectID),
		zap.Stringer("message_id", message.ID),
	)

	return g.Fail(ctx, message, providers.ReasonOptedOut, errRecipientOptedOut)
}

// SendFailed classifies a provider rejection and settles the message when the
// rejection is terminal. It returns the error the caller should propagate.
//
// A provider reporting ReasonOptedOut is telling us a fact about consent we did
// not have, so it is written to the suppression ledger before the message is
// settled. That write is best effort: losing it costs one more rejected send
// (the provider reports the same fact again), whereas losing the settle would
// strand the message and hang its broadcast.
func (g *complianceGate) SendFailed(ctx context.Context, message *subjects.InboxMessage, to string, sendErr error) error {
	var providerErr *wasmProviders.ProviderError
	if !errors.As(sendErr, &providerErr) {
		return sendErr
	}

	reason := providerErr.Reason
	if !reason.Valid() {
		reason = providers.ReasonUnknown
	}

	if reason == providers.ReasonOptedOut {
		g.recordProviderOptOut(ctx, message, to, providerErr)
	}

	// A recipient who opted out does not opt back in by being retried, so an
	// opt-out is terminal whether or not the module flagged it permanent.
	if !providerErr.IsPermanent() && reason != providers.ReasonOptedOut {
		return sendErr
	}

	return g.Fail(ctx, message, reason, sendErr)
}

// Fail settles a message that will never be delivered and returns the error the
// dispatch caller should propagate: permanent once the outcome is recorded, and
// transient while it could not be, so the settle is retried rather than lost.
func (g *complianceGate) Fail(ctx context.Context, message *subjects.InboxMessage, reason providers.FailureReason, cause error) error {
	marked, err := g.markFailed(ctx, message, reason)
	if err != nil {
		if IsPermanent(err) {
			return err
		}
		g.logger.Error("failed to settle inbox message", zap.Error(err), zap.Stringer("message_id", message.ID))
		return fmt.Errorf("settle inbox message %s as %s: %w", message.ID, reason, err)
	}

	if marked {
		if err := PublishInboxOutcome(ctx, g.pub, message, schemas.EventInboxMessageFailed); err != nil {
			// Non-fatal: failed_at is persisted and the event can be replayed.
			g.logger.Error("failed to publish inbox failed event", zap.Error(err), zap.Stringer("message_id", message.ID))
		}
	}

	return Permanent(fmt.Errorf("inbox message %s failed permanently (%s): %w", message.ID, reason, cause))
}

func (g *complianceGate) markFailed(ctx context.Context, message *subjects.InboxMessage, reason providers.FailureReason) (bool, error) {
	switch {
	case message.UserID != nil:
		return g.outcomes.MarkUserInboxMessageFailed(ctx, message.ID, string(reason))
	case message.OrganizationID != nil:
		return g.outcomes.MarkOrganizationInboxMessageFailed(ctx, message.ID, string(reason))
	default:
		return false, Permanentf("inbox message %s has neither user_id nor organization_id", message.ID)
	}
}

func (g *complianceGate) recordProviderOptOut(ctx context.Context, message *subjects.InboxMessage, to string, providerErr *wasmProviders.ProviderError) {
	err := g.suppressions.RecordOptOut(ctx, management.SuppressionInput{
		ProjectID:      message.ProjectID,
		RecipientPhone: to,
		Source:         management.ConsentSourceProvider,
		Reason:         providerErr.Error(),
	})
	if err != nil {
		g.logger.Error("failed to record provider-reported opt-out",
			zap.Error(err),
			zap.Stringer("project_id", message.ProjectID),
			zap.Stringer("message_id", message.ID),
		)
	}
}

// dispatchRecipient returns the address the gate must decide on. For SMS that is
// the number in the composed payload, which is the one that will actually be
// dialled: organisation dispatch hands DispatchDirect an empty recipient and
// carries the real one in the payload, and gating on the empty string would
// refuse every organisation SMS as unusable.
func dispatchRecipient(channel providers.Channel, payload json.RawMessage, fallback string) string {
	if channel != providers.ChannelSMS {
		return fallback
	}

	var sms providers.SMSPayload
	if err := json.Unmarshal(payload, &sms); err == nil && sms.To != "" {
		return sms.To
	}

	return fallback
}

// inboxRecipientTimezone returns the timezone to persist on a new inbox
// message: the recipient's resolved zone for SMS, and nil for every other
// channel and whenever nothing resolved.
//
// The zone is recorded, never enforced - nothing on the send path reads it yet.
// It exists so a later quiet-hours rollout can be measured against real
// history, which is why an unresolved recipient leaves the column NULL rather
// than storing a guess, and why resolution can never fail a send.
func inboxRecipientTimezone(channel modules.Channel, in compliance.RecipientTimezoneInput) *string {
	if providers.Channel(channel) != providers.ChannelSMS {
		return nil
	}

	zone, _ := compliance.ResolveRecipientTimezone(in)
	if zone == "" {
		return nil
	}

	return ptr.To(zone)
}
