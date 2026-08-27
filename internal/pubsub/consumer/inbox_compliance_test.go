package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/compliance"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	wasmProviders "github.com/lunogram/platform/internal/wasm/providers"
	"github.com/lunogram/platform/pkg/modules"
	providers "github.com/lunogram/platform/pkg/modules/providers"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type fakeSuppressions struct {
	suppressed bool
	lookupErr  error
	lookups    []string

	optOuts   []management.SuppressionInput
	optOutErr error
}

func (f *fakeSuppressions) IsSuppressed(_ context.Context, _ uuid.UUID, recipientPhone string) (bool, error) {
	f.lookups = append(f.lookups, recipientPhone)
	if f.lookupErr != nil {
		return false, f.lookupErr
	}
	return f.suppressed, nil
}

func (f *fakeSuppressions) RecordOptOut(_ context.Context, in management.SuppressionInput) error {
	f.optOuts = append(f.optOuts, in)
	return f.optOutErr
}

type fakeOutcomes struct {
	userFailed map[uuid.UUID]string
	orgFailed  map[uuid.UUID]string
	alreadySet bool
	err        error
}

func newFakeOutcomes() *fakeOutcomes {
	return &fakeOutcomes{userFailed: map[uuid.UUID]string{}, orgFailed: map[uuid.UUID]string{}}
}

func (f *fakeOutcomes) MarkUserInboxMessageFailed(_ context.Context, id uuid.UUID, reason string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	f.userFailed[id] = reason
	return !f.alreadySet, nil
}

func (f *fakeOutcomes) MarkOrganizationInboxMessageFailed(_ context.Context, id uuid.UUID, reason string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	f.orgFailed[id] = reason
	return !f.alreadySet, nil
}

func newTestGate(t *testing.T, suppressions *fakeSuppressions, outcomes *fakeOutcomes, pub *fakePublisher) *complianceGate {
	t.Helper()
	return &complianceGate{
		suppressions: suppressions,
		outcomes:     outcomes,
		pub:          pub,
		logger:       zaptest.NewLogger(t),
	}
}

func newTestSMS(t *testing.T, class string) *subjects.InboxMessage {
	t.Helper()
	userID := uuid.New()
	return &subjects.InboxMessage{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		UserID:    &userID,
		Channel:   modules.Channel(providers.ChannelSMS),
		Class:     class,
	}
}

// TestGateBlocksSuppressedStandardSMS covers the core promise: a standard SMS to
// an opted-out recipient is settled as failed and the caller is handed a
// permanent error, so DispatchDirect returns before it ever calls Send.
func TestGateBlocksSuppressedStandardSMS(t *testing.T) {
	t.Parallel()

	suppressions := &fakeSuppressions{suppressed: true}
	outcomes := newFakeOutcomes()
	pub := &fakePublisher{}
	gate := newTestGate(t, suppressions, outcomes, pub)
	message := newTestSMS(t, subjects.InboxClassStandard)

	err := gate.Allow(t.Context(), message, "+14155552671")

	require.Error(t, err)
	require.True(t, IsPermanent(err), "a suppressed send must not be retried")
	require.Equal(t, []string{"+14155552671"}, suppressions.lookups)
	require.Equal(t, string(providers.ReasonOptedOut), outcomes.userFailed[message.ID])
	require.Empty(t, outcomes.orgFailed)
	require.Contains(t, pub.subjects(), schemas.UserEventsProcess(message.ProjectID))
	require.Contains(t, pub.subjects(), schemas.UserInboxFailed(message.ProjectID))
}

// TestGateAllowsSuppressedComplianceSMS covers the mandated bypass: an opt-out
// confirmation and a HELP reply have to reach someone who has just opted out, so
// a compliance-class message is handed to the provider even while suppressed.
func TestGateAllowsSuppressedComplianceSMS(t *testing.T) {
	t.Parallel()

	suppressions := &fakeSuppressions{suppressed: true}
	outcomes := newFakeOutcomes()
	pub := &fakePublisher{}
	gate := newTestGate(t, suppressions, outcomes, pub)
	message := newTestSMS(t, subjects.InboxClassCompliance)

	require.NoError(t, gate.Allow(t.Context(), message, "+14155552671"))
	require.Empty(t, outcomes.userFailed)
	require.Empty(t, pub.published)
}

// TestGateIgnoresEmail asserts the gate is SMS-only: an email is never even
// looked up, let alone suppressed.
func TestGateIgnoresEmail(t *testing.T) {
	t.Parallel()

	suppressions := &fakeSuppressions{suppressed: true}
	outcomes := newFakeOutcomes()
	pub := &fakePublisher{}
	gate := newTestGate(t, suppressions, outcomes, pub)

	userID := uuid.New()
	message := &subjects.InboxMessage{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		UserID:    &userID,
		Channel:   modules.Channel(providers.ChannelEmail),
		Class:     subjects.InboxClassStandard,
	}

	require.NoError(t, gate.Allow(t.Context(), message, "user@example.com"))
	require.Empty(t, suppressions.lookups)
	require.Empty(t, outcomes.userFailed)
}

// TestGateNeverFailsOpen asserts the safety property: when suppression state
// cannot be read, the send is refused and retried rather than let through.
func TestGateNeverFailsOpen(t *testing.T) {
	t.Parallel()

	lookupErr := errors.New("connection refused")
	suppressions := &fakeSuppressions{lookupErr: lookupErr}
	outcomes := newFakeOutcomes()
	pub := &fakePublisher{}
	gate := newTestGate(t, suppressions, outcomes, pub)
	message := newTestSMS(t, subjects.InboxClassStandard)

	err := gate.Allow(t.Context(), message, "+14155552671")

	require.Error(t, err, "an unreadable suppression state must never be read as consent")
	require.ErrorIs(t, err, lookupErr)
	require.False(t, IsPermanent(err), "infrastructure failures must be retried")
	require.Empty(t, outcomes.userFailed, "a retryable lookup failure is not a terminal outcome")
}

// TestGateTreatsInvalidNumberAsPermanent asserts that a recipient we cannot
// normalise is settled rather than sent to anyway.
func TestGateTreatsInvalidNumberAsPermanent(t *testing.T) {
	t.Parallel()

	suppressions := &fakeSuppressions{lookupErr: fmt.Errorf("%w: %q", management.ErrInvalidPhoneNumber, "nope")}
	outcomes := newFakeOutcomes()
	pub := &fakePublisher{}
	gate := newTestGate(t, suppressions, outcomes, pub)
	message := newTestSMS(t, subjects.InboxClassStandard)

	err := gate.Allow(t.Context(), message, "nope")

	require.True(t, IsPermanent(err))
	require.Equal(t, string(providers.ReasonInvalidNumber), outcomes.userFailed[message.ID])
}

// TestGateSettlesOrganizationMessagesOnTheirOwnTable asserts DispatchDirect's
// shared gate writes to the table that owns the message.
func TestGateSettlesOrganizationMessagesOnTheirOwnTable(t *testing.T) {
	t.Parallel()

	suppressions := &fakeSuppressions{suppressed: true}
	outcomes := newFakeOutcomes()
	pub := &fakePublisher{}
	gate := newTestGate(t, suppressions, outcomes, pub)

	orgID := uuid.New()
	message := &subjects.InboxMessage{
		ID:             uuid.New(),
		ProjectID:      uuid.New(),
		OrganizationID: &orgID,
		Channel:        modules.Channel(providers.ChannelSMS),
		Class:          subjects.InboxClassStandard,
	}

	require.True(t, IsPermanent(gate.Allow(t.Context(), message, "+14155552671")))
	require.Equal(t, string(providers.ReasonOptedOut), outcomes.orgFailed[message.ID])
	require.Empty(t, outcomes.userFailed)
	require.Contains(t, pub.subjects(), schemas.OrganizationInboxFailed(message.ProjectID))
}

// TestGateRetriesWhenTheSettleFails asserts the settle is never dropped: if the
// terminal outcome cannot be written the whole delivery is retried, because a
// message with neither sent_at nor failed_at would hang its broadcast.
func TestGateRetriesWhenTheSettleFails(t *testing.T) {
	t.Parallel()

	outcomes := newFakeOutcomes()
	outcomes.err = errors.New("connection refused")
	pub := &fakePublisher{}
	gate := newTestGate(t, &fakeSuppressions{suppressed: true}, outcomes, pub)
	message := newTestSMS(t, subjects.InboxClassStandard)

	err := gate.Allow(t.Context(), message, "+14155552671")

	require.Error(t, err)
	require.False(t, IsPermanent(err))
	require.Empty(t, pub.published)
}

// TestGateDoesNotRepublishAnAlreadySettledMessage mirrors the sent path's
// idempotency guard: a redelivery that finds failed_at already set must not
// emit a second terminal event.
func TestGateDoesNotRepublishAnAlreadySettledMessage(t *testing.T) {
	t.Parallel()

	outcomes := newFakeOutcomes()
	outcomes.alreadySet = true
	pub := &fakePublisher{}
	gate := newTestGate(t, &fakeSuppressions{suppressed: true}, outcomes, pub)
	message := newTestSMS(t, subjects.InboxClassStandard)

	require.True(t, IsPermanent(gate.Allow(t.Context(), message, "+14155552671")))
	require.Empty(t, pub.published)
}

// TestSendFailedBackfillsProviderReportedOptOut covers task 2: the provider knew
// something we did not, so the fact is written to the consent ledger with
// provider provenance and the message settles.
func TestSendFailedBackfillsProviderReportedOptOut(t *testing.T) {
	t.Parallel()

	suppressions := &fakeSuppressions{}
	outcomes := newFakeOutcomes()
	pub := &fakePublisher{}
	gate := newTestGate(t, suppressions, outcomes, pub)
	message := newTestSMS(t, subjects.InboxClassStandard)

	sendErr := &wasmProviders.ProviderError{
		Code:    0xFFFFFFFE,
		Message: "The message From/To pair violates a blacklist rule",
		Reason:  providers.ReasonOptedOut,
	}

	err := gate.SendFailed(t.Context(), message, "+14155552671", sendErr)

	require.True(t, IsPermanent(err))
	require.Len(t, suppressions.optOuts, 1)

	recorded := suppressions.optOuts[0]
	require.Equal(t, message.ProjectID, recorded.ProjectID)
	require.Equal(t, "+14155552671", recorded.RecipientPhone)
	require.Equal(t, management.ConsentSourceProvider, recorded.Source)
	require.Nil(t, recorded.InboundID)
	require.Equal(t, sendErr.Message, recorded.Reason)

	require.Equal(t, string(providers.ReasonOptedOut), outcomes.userFailed[message.ID])
	require.Contains(t, pub.subjects(), schemas.UserInboxFailed(message.ProjectID))
}

// TestSendFailedSettlesEvenWhenTheSuppressionWriteFails records the ordering
// choice: the suppression row is attempted first but cannot veto the settle,
// because the provider will report the same opt-out again while a stranded
// message would hang its broadcast forever.
func TestSendFailedSettlesEvenWhenTheSuppressionWriteFails(t *testing.T) {
	t.Parallel()

	suppressions := &fakeSuppressions{optOutErr: errors.New("connection refused")}
	outcomes := newFakeOutcomes()
	gate := newTestGate(t, suppressions, outcomes, &fakePublisher{})
	message := newTestSMS(t, subjects.InboxClassStandard)

	err := gate.SendFailed(t.Context(), message, "+14155552671", &wasmProviders.ProviderError{
		Code:   0xFFFFFFFE,
		Reason: providers.ReasonOptedOut,
	})

	require.True(t, IsPermanent(err))
	require.Equal(t, string(providers.ReasonOptedOut), outcomes.userFailed[message.ID])
}

func TestSendFailedRecordsCanonicalReasons(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err       error
		permanent bool
		reason    string
	}{
		"permanent invalid number": {
			err:       &wasmProviders.ProviderError{Code: 0xFFFFFFFE, Reason: providers.ReasonInvalidNumber},
			permanent: true,
			reason:    string(providers.ReasonInvalidNumber),
		},
		"permanent unregistered sender": {
			err:       &wasmProviders.ProviderError{Code: 0xFFFFFFFE, Reason: providers.ReasonUnregistered},
			permanent: true,
			reason:    string(providers.ReasonUnregistered),
		},
		"an unclassified permanent failure lands as unknown, not as a raw provider string": {
			err:       &wasmProviders.ProviderError{Code: 0xFFFFFFFE, Message: "HTTP 400: nope"},
			permanent: true,
			reason:    string(providers.ReasonUnknown),
		},
		"a throttle stays retryable and does not settle": {
			err:       &wasmProviders.ProviderError{Code: 1, Reason: providers.ReasonRateLimited},
			permanent: false,
		},
		"a plain transport error stays retryable": {
			err:       errors.New("dial tcp: i/o timeout"),
			permanent: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			outcomes := newFakeOutcomes()
			gate := newTestGate(t, &fakeSuppressions{}, outcomes, &fakePublisher{})
			message := newTestSMS(t, subjects.InboxClassStandard)

			err := gate.SendFailed(t.Context(), message, "+14155552671", tc.err)
			require.Error(t, err)

			if !tc.permanent {
				require.False(t, IsPermanent(err))
				require.Empty(t, outcomes.userFailed)
				return
			}

			require.True(t, IsPermanent(err))
			require.Equal(t, tc.reason, outcomes.userFailed[message.ID])
		})
	}
}

// TestDispatchRecipientPrefersThePayloadAddress covers the organisation path,
// where DispatchDirect is handed an empty "to" and the real recipient lives in
// the composed payload. Gating on the caller-supplied address alone would refuse
// every organisation SMS as unusable.
func TestDispatchRecipientPrefersThePayloadAddress(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(providers.SMSPayload{To: "+14155552671", From: "+15005550006", Body: "hi"})
	require.NoError(t, err)

	require.Equal(t, "+14155552671", dispatchRecipient(providers.ChannelSMS, payload, ""))
	require.Equal(t, "+14155552671", dispatchRecipient(providers.ChannelSMS, payload, "+31612345678"))
	require.Equal(t, "+31612345678", dispatchRecipient(providers.ChannelSMS, json.RawMessage(`{"subject":"hi"}`), "+31612345678"))
	require.Equal(t, "+31612345678", dispatchRecipient(providers.ChannelSMS, json.RawMessage(`not json`), "+31612345678"))

	// An email payload also carries a "to", which must never be mistaken for a
	// number the suppression ledger could be keyed on.
	email, err := json.Marshal(providers.EmailPayload{To: "user@example.com", Subject: "hi"})
	require.NoError(t, err)
	require.Equal(t, "fallback@example.com", dispatchRecipient(providers.ChannelEmail, email, "fallback@example.com"))
}

func TestInboxRecipientTimezone(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		channel providers.Channel
		input   compliance.RecipientTimezoneInput
		zone    *string
	}{
		"an SMS records the recipient's own zone": {
			channel: providers.ChannelSMS,
			input:   compliance.RecipientTimezoneInput{UserTimezone: "Australia/Sydney", ProjectTimezone: "UTC"},
			zone:    ptr.To("Australia/Sydney"),
		},
		"an SMS falls back to the number's prefix": {
			channel: providers.ChannelSMS,
			input:   compliance.RecipientTimezoneInput{Phone: "+14155552671", ProjectTimezone: "UTC"},
			zone:    ptr.To("America/Los_Angeles"),
		},
		"an SMS falls back to the project": {
			channel: providers.ChannelSMS,
			input:   compliance.RecipientTimezoneInput{ProjectTimezone: "Europe/Amsterdam"},
			zone:    ptr.To("Europe/Amsterdam"),
		},
		"an unresolvable recipient leaves the column NULL": {
			channel: providers.ChannelSMS,
			input:   compliance.RecipientTimezoneInput{Phone: "nope"},
			zone:    nil,
		},
		"email is not an SMS and resolves to nothing": {
			channel: providers.ChannelEmail,
			input:   compliance.RecipientTimezoneInput{UserTimezone: "Australia/Sydney"},
			zone:    nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.zone, inboxRecipientTimezone(modules.Channel(tc.channel), tc.input))
		})
	}
}
