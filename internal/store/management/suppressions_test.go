package management

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSuppressionsStore(t *testing.T) {
	t.Parallel()
	db, raw := newContainerStoreWithDB(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	newProject := func(name string) uuid.UUID {
		t.Helper()
		id, err := db.CreateProject(ctx, Project{
			OrganizationID: &orgID,
			Name:           name,
			Timezone:       "UTC",
			Locale:         "en",
		})
		require.NoError(t, err)
		return id
	}

	projectID := newProject("Suppressions Project")

	t.Run("rejects a malformed phone number", func(t *testing.T) {
		err := db.RecordOptOut(ctx, SuppressionInput{
			ProjectID:      projectID,
			RecipientPhone: "not-a-number",
			Source:         ConsentSourceInboundSMS,
			Reason:         "STOP",
		})
		require.ErrorIs(t, err, ErrInvalidPhoneNumber)

		_, err = db.IsSuppressed(ctx, projectID, "12345")
		require.ErrorIs(t, err, ErrInvalidPhoneNumber)
	})

	t.Run("rejects an empty or unrecognised consent source", func(t *testing.T) {
		for _, source := range []ConsentSource{"", "smoke_signal", "INBOUND_SMS"} {
			err := db.RecordOptOut(ctx, SuppressionInput{
				ProjectID:      projectID,
				RecipientPhone: "+14155559999",
				Source:         source,
				Reason:         "STOP keyword",
			})
			require.ErrorIsf(t, err, ErrInvalidConsentSource, "source %q must be rejected", source)
		}

		rows, err := countSuppressionRows(ctx, raw, projectID, "+14155559999")
		require.NoError(t, err)
		assert.Zero(t, rows, "a rejected source must not write a suppression row")
	})

	t.Run("records provider-reported opt-out as provider, not api", func(t *testing.T) {
		phone := "+14155552676"

		require.NoError(t, db.RecordOptOut(ctx, SuppressionInput{
			ProjectID:      projectID,
			RecipientPhone: phone,
			Source:         ConsentSourceProvider,
			Reason:         "twilio 21610",
		}))

		var source string
		require.NoError(t, raw.GetContext(ctx, &source, `
			SELECT source FROM sms_consent_events
			WHERE project_id = $1 AND recipient_phone = $2`, projectID, phone))
		assert.Equal(t, string(ConsentSourceProvider), source)
	})

	t.Run("normalises to E.164 before storing and looking up", func(t *testing.T) {
		require.NoError(t, db.RecordOptOut(ctx, SuppressionInput{
			ProjectID:      projectID,
			RecipientPhone: "+1 (415) 555-2671",
			Source:         ConsentSourceInboundSMS,
			Reason:         "STOP keyword",
		}))

		stored, err := suppressionRow(ctx, raw, projectID, "+14155552671")
		require.NoError(t, err)
		assert.Equal(t, "+14155552671", stored.RecipientPhone)
		assert.Equal(t, SuppressionScopeAll, stored.SenderAddress)

		for _, variant := range []string{"+14155552671", "+1 415 555 2671", "+1-415-555-2671"} {
			suppressed, err := db.IsSuppressed(ctx, projectID, variant)
			require.NoError(t, err)
			assert.Truef(t, suppressed, "expected %q to resolve to the stored suppression", variant)
		}
	})

	t.Run("repeated opt-out is idempotent and keeps appending to the ledger", func(t *testing.T) {
		phone := "+14155552672"
		in := SuppressionInput{
			ProjectID:      projectID,
			RecipientPhone: phone,
			Source:         ConsentSourceInboundSMS,
			Reason:         "STOP keyword",
		}

		require.NoError(t, db.RecordOptOut(ctx, in))
		require.NoError(t, db.RecordOptOut(ctx, in))
		require.NoError(t, db.RecordOptOut(ctx, in))

		suppressed, err := db.IsSuppressed(ctx, projectID, phone)
		require.NoError(t, err)
		assert.True(t, suppressed)

		rows, err := countSuppressionRows(ctx, raw, projectID, phone)
		require.NoError(t, err)
		assert.Equal(t, 1, rows)

		events, err := countConsentEvents(ctx, raw, projectID, phone, "opt_out")
		require.NoError(t, err)
		assert.Equal(t, 3, events)
	})

	t.Run("opt-in reverses a previous opt-out", func(t *testing.T) {
		phone := "+14155552673"

		require.NoError(t, db.RecordOptOut(ctx, SuppressionInput{
			ProjectID:      projectID,
			RecipientPhone: phone,
			Source:         ConsentSourceInboundSMS,
			Reason:         "STOP keyword",
		}))

		suppressed, err := db.IsSuppressed(ctx, projectID, phone)
		require.NoError(t, err)
		require.True(t, suppressed)

		require.NoError(t, db.RecordOptIn(ctx, SuppressionInput{
			ProjectID:      projectID,
			RecipientPhone: phone,
			Source:         ConsentSourcePreferenceCenter,
			Reason:         "START keyword",
		}))

		suppressed, err = db.IsSuppressed(ctx, projectID, phone)
		require.NoError(t, err)
		assert.False(t, suppressed)

		rows, err := countSuppressionRows(ctx, raw, projectID, phone)
		require.NoError(t, err)
		assert.Zero(t, rows, "opting back in removes the suppression row rather than flagging it")

		optOuts, err := countConsentEvents(ctx, raw, projectID, phone, "opt_out")
		require.NoError(t, err)
		assert.Equal(t, 1, optOuts)

		optIns, err := countConsentEvents(ctx, raw, projectID, phone, "opt_in")
		require.NoError(t, err)
		assert.Equal(t, 1, optIns)
	})

	t.Run("suppression is scoped to a single project", func(t *testing.T) {
		other := newProject("Other Suppressions Project")
		phone := "+14155552674"

		require.NoError(t, db.RecordOptOut(ctx, SuppressionInput{
			ProjectID:      projectID,
			RecipientPhone: phone,
			Source:         ConsentSourceInboundSMS,
			Reason:         "STOP keyword",
		}))

		suppressed, err := db.IsSuppressed(ctx, projectID, phone)
		require.NoError(t, err)
		assert.True(t, suppressed)

		suppressed, err = db.IsSuppressed(ctx, other, phone)
		require.NoError(t, err)
		assert.False(t, suppressed)
	})

	t.Run("records the inbound message that carried the signal", func(t *testing.T) {
		phone := "+14155552675"
		inbound := uuid.New()

		require.NoError(t, db.RecordOptOut(ctx, SuppressionInput{
			ProjectID:      projectID,
			RecipientPhone: phone,
			Source:         ConsentSourceInboundSMS,
			Reason:         "STOP keyword",
			InboundID:      &inbound,
		}))

		var source string
		require.NoError(t, raw.GetContext(ctx, &source, `
			SELECT source FROM sms_consent_events
			WHERE project_id = $1 AND recipient_phone = $2 AND inbound_id = $3`,
			projectID, phone, inbound))
		assert.Equal(t, "inbound_sms", source)
	})
}

type storedSuppression struct {
	SenderAddress  string `db:"sender_address"`
	RecipientPhone string `db:"recipient_phone"`
	Reason         string `db:"reason"`
}

func suppressionRow(ctx context.Context, db store.DB, projectID uuid.UUID, phone string) (storedSuppression, error) {
	var row storedSuppression
	err := db.GetContext(ctx, &row, `
		SELECT sender_address, recipient_phone, reason
		FROM sms_suppressions
		WHERE project_id = $1 AND recipient_phone = $2`, projectID, phone)
	return row, err
}

func countSuppressionRows(ctx context.Context, db store.DB, projectID uuid.UUID, phone string) (int, error) {
	var count int
	err := db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM sms_suppressions
		WHERE project_id = $1 AND recipient_phone = $2`, projectID, phone)
	return count, err
}

func countConsentEvents(ctx context.Context, db store.DB, projectID uuid.UUID, phone, transition string) (int, error) {
	var count int
	err := db.GetContext(ctx, &count, `
		SELECT COUNT(*) FROM sms_consent_events
		WHERE project_id = $1 AND recipient_phone = $2 AND transition = $3`, projectID, phone, transition)
	return count, err
}
