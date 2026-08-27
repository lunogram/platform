package management

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/store"
	"github.com/nyaruka/phonenumbers"
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

// IsSuppressed reports whether the recipient is suppressed for this project.
//
// The lookup does not constrain sender_address: while every row is written
// project-wide any row for the recipient suppresses the send, so a future
// per-sender row fails closed rather than opening a gap.
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
	)`

	var suppressed bool
	if err := s.db.GetContext(ctx, &suppressed, stmt, projectID, phone); err != nil {
		return false, err
	}

	return suppressed, nil
}

// RecordOptOut adds the recipient to the project's suppression set and appends
// a consent event.
//
// Repeating a STOP is not an error: the upsert converges on the same row,
// while the ledger still gains an entry because it records every consent
// signal received, not every change of state. A dispute asks when a STOP
// arrived and how often, which a deduplicated log could not answer.
func (s *SuppressionsStore) RecordOptOut(ctx context.Context, in SuppressionInput) error {
	target, err := s.resolve(in)
	if err != nil {
		return err
	}

	stmt := `
	WITH suppression AS (
		INSERT INTO sms_suppressions (project_id, sender_address, recipient_phone, reason)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (project_id, sender_address, recipient_phone) DO UPDATE
		SET reason = EXCLUDED.reason,
			occurred_at = NOW()
	)
	INSERT INTO sms_consent_events (project_id, recipient_phone, sender_address, transition, source, inbound_id)
	VALUES ($1, $3, $2, $5, $6, $7)`

	_, err = s.db.ExecContext(ctx, stmt,
		target.projectID,
		target.sender,
		target.phone,
		in.Reason,
		consentTransitionOptOut,
		string(in.Source),
		in.InboundID,
	)

	return err
}

// RecordOptIn removes the recipient from the project's suppression set and
// appends a consent event.
//
// The ledger entry does not depend on a row having been deleted: a START from
// a number that was never suppressed is still a consent signal we received,
// and the record of receiving it is the point of the ledger.
func (s *SuppressionsStore) RecordOptIn(ctx context.Context, in SuppressionInput) error {
	target, err := s.resolve(in)
	if err != nil {
		return err
	}

	stmt := `
	WITH suppression AS (
		DELETE FROM sms_suppressions
		WHERE project_id = $1
		AND sender_address = $2
		AND recipient_phone = $3
	)
	INSERT INTO sms_consent_events (project_id, recipient_phone, sender_address, transition, source, inbound_id)
	VALUES ($1, $3, $2, $4, $5, $6)`

	_, err = s.db.ExecContext(ctx, stmt,
		target.projectID,
		target.sender,
		target.phone,
		consentTransitionOptIn,
		string(in.Source),
		in.InboundID,
	)

	return err
}

type suppressionTarget struct {
	projectID uuid.UUID
	sender    string
	phone     string
}

// resolve validates the consent source and normalises the addressing fields
// shared by both transitions.
func (s *SuppressionsStore) resolve(in SuppressionInput) (suppressionTarget, error) {
	if _, ok := consentSources[in.Source]; !ok {
		return suppressionTarget{}, fmt.Errorf("%w: %q", ErrInvalidConsentSource, in.Source)
	}

	phone, err := normalizeE164(in.RecipientPhone)
	if err != nil {
		return suppressionTarget{}, err
	}

	sender := in.SenderAddress
	if sender == "" {
		sender = SuppressionScopeAll
	}

	return suppressionTarget{projectID: in.ProjectID, sender: sender, phone: phone}, nil
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
