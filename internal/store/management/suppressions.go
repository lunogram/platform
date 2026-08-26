package management

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/store"
	"github.com/nyaruka/phonenumbers"
)

type SuppressionState string

const (
	SuppressionOptedOut SuppressionState = "opted_out"
	SuppressionOptedIn  SuppressionState = "opted_in"
)

// SuppressionScopeAll is the sentinel sender address used while opt-out
// scope is project-wide.
const SuppressionScopeAll = "*"

const (
	consentTransitionOptOut = "opt_out"
	consentTransitionOptIn  = "opt_in"
)

// ConsentSource identifies how a consent signal reached us. The consent ledger
// is the record produced in a dispute, so provenance is never inferred: a
// provider-reported opt-out and a support agent acting on a phone call are
// different facts and must not be recorded as the same one.
type ConsentSource string

const (
	ConsentSourceInboundSMS       ConsentSource = "inbound_sms"
	ConsentSourceProvider         ConsentSource = "provider"
	ConsentSourceAPI              ConsentSource = "api"
	ConsentSourceManual           ConsentSource = "manual"
	ConsentSourcePreferenceCenter ConsentSource = "preference_center"
)

var consentSources = map[ConsentSource]struct{}{
	ConsentSourceInboundSMS:       {},
	ConsentSourceProvider:         {},
	ConsentSourceAPI:              {},
	ConsentSourceManual:           {},
	ConsentSourcePreferenceCenter: {},
}

// ErrInvalidPhoneNumber is returned when a recipient phone number cannot be
// normalised to E.164. Storing the number as given would create a suppression
// row that no lookup can ever match, silently disarming the opt-out.
var ErrInvalidPhoneNumber = errors.New("management: recipient phone number is not a valid E.164 number")

// ErrInvalidConsentSource is returned when a consent signal carries no source
// or one outside the known set. There is no default: guessing the provenance
// would write a plausible falsehood into the ledger a dispute is settled from.
var ErrInvalidConsentSource = errors.New("management: consent source is empty or unrecognised")

type SuppressionInput struct {
	ProjectID      uuid.UUID
	RecipientPhone string
	SenderAddress  string
	Source         ConsentSource
	Reason         string
	InboundID      *uuid.UUID
}

func NewSuppressionsStore(db store.DB) *SuppressionsStore {
	return &SuppressionsStore{db: db}
}

type SuppressionsStore struct {
	db store.DB
}

// IsSuppressed reports whether the recipient is opted out for this project.
//
// The lookup does not constrain sender_address: while every row is written
// project-wide any opted-out row for the recipient suppresses the send, so a
// future per-sender row fails closed rather than opening a gap.
func (s *SuppressionsStore) IsSuppressed(ctx context.Context, projectID uuid.UUID, recipientPhone string) (bool, error) {
	phone, err := normalizeE164(recipientPhone)
	if err != nil {
		return false, err
	}

	stmt := `
	SELECT EXISTS(
		SELECT 1
		FROM sms_suppressions
		WHERE project_id = $1
		AND recipient_phone = $2
		AND state = 'opted_out'
	)`

	var suppressed bool
	if err := s.db.GetContext(ctx, &suppressed, stmt, projectID, phone); err != nil {
		return false, err
	}

	return suppressed, nil
}

// RecordOptOut upserts suppression state to opted_out and appends a consent event.
func (s *SuppressionsStore) RecordOptOut(ctx context.Context, in SuppressionInput) error {
	return s.record(ctx, in, SuppressionOptedOut, consentTransitionOptOut)
}

// RecordOptIn upserts suppression state to opted_in and appends a consent event.
func (s *SuppressionsStore) RecordOptIn(ctx context.Context, in SuppressionInput) error {
	return s.record(ctx, in, SuppressionOptedIn, consentTransitionOptIn)
}

// record settles the current suppression state and appends to the consent
// ledger in one statement, so the two can never disagree.
//
// Repeating a transition is not an error: the upsert converges on the same
// state, while the ledger still gains a row because it records every consent
// signal received, not every change of state. A dispute asks when a STOP
// arrived and how often, which a deduplicated log could not answer.
func (s *SuppressionsStore) record(ctx context.Context, in SuppressionInput, state SuppressionState, transition string) error {
	if _, ok := consentSources[in.Source]; !ok {
		return fmt.Errorf("%w: %q", ErrInvalidConsentSource, in.Source)
	}

	phone, err := normalizeE164(in.RecipientPhone)
	if err != nil {
		return err
	}

	sender := in.SenderAddress
	if sender == "" {
		sender = SuppressionScopeAll
	}

	stmt := `
	WITH suppression AS (
		INSERT INTO sms_suppressions (project_id, sender_address, recipient_phone, state, reason, source_message_id)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (project_id, sender_address, recipient_phone) DO UPDATE
		SET state = EXCLUDED.state,
			reason = EXCLUDED.reason,
			source_message_id = EXCLUDED.source_message_id,
			occurred_at = NOW()
		RETURNING project_id, sender_address, recipient_phone
	)
	INSERT INTO sms_consent_events (project_id, recipient_phone, sender_address, transition, source, inbound_id)
	SELECT project_id, recipient_phone, sender_address, $7, $8, $6
	FROM suppression`

	_, err = s.db.ExecContext(ctx, stmt,
		in.ProjectID,
		sender,
		phone,
		string(state),
		in.Reason,
		in.InboundID,
		transition,
		string(in.Source),
	)

	return err
}

func normalizeE164(phone string) (string, error) {
	number, err := phonenumbers.Parse(phone, "")
	if err != nil {
		return "", fmt.Errorf("%w: %q: %w", ErrInvalidPhoneNumber, phone, err)
	}

	if !phonenumbers.IsValidNumber(number) {
		return "", fmt.Errorf("%w: %q", ErrInvalidPhoneNumber, phone)
	}

	return phonenumbers.Format(number, phonenumbers.E164), nil
}
