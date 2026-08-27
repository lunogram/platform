package subjects

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/stretchr/testify/require"
)

// TestCreateInboxMessageClassAndRecipientTimezone covers the two columns the
// send path added. Class matters most: it is what lets an opt-out confirmation
// past the suppression gate, so a caller that cannot set it at insert leaves
// that bypass unreachable.
func TestCreateInboxMessageClassAndRecipientTimezone(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	ctx := context.Background()
	projectID := uuid.New()

	userID, err := db.UpsertUser(ctx, projectID, UpsertUserParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "user-class"}},
	})
	require.NoError(t, err)

	organizationID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "org-class"}},
	})
	require.NoError(t, err)

	senderIdentityID := uuid.New()

	standard, err := db.CreateUserInboxMessage(ctx, projectID, userID, InboxMessageParams{
		Channel:          "sms",
		SenderIdentityID: &senderIdentityID,
		Content:          titleContent("Standard"),
	})
	require.NoError(t, err)
	require.Equal(t, InboxClassStandard, standard.Class, "an unset class must default rather than reject")
	require.Nil(t, standard.RecipientTimezone, "an unresolved zone stays NULL rather than being guessed")

	compliance, err := db.CreateUserInboxMessage(ctx, projectID, userID, InboxMessageParams{
		Channel:           "sms",
		SenderIdentityID:  &senderIdentityID,
		Content:           titleContent("You have been unsubscribed"),
		Class:             InboxClassCompliance,
		RecipientTimezone: ptr.To("America/Los_Angeles"),
	})
	require.NoError(t, err)
	require.Equal(t, InboxClassCompliance, compliance.Class)
	require.Equal(t, ptr.To("America/Los_Angeles"), compliance.RecipientTimezone)

	organization, err := db.CreateOrganizationInboxMessage(ctx, projectID, organizationID, InboxMessageParams{
		Channel:           "sms",
		SenderIdentityID:  &senderIdentityID,
		Content:           titleContent("You have been unsubscribed"),
		Class:             InboxClassCompliance,
		RecipientTimezone: ptr.To("Europe/Amsterdam"),
	})
	require.NoError(t, err)
	require.Equal(t, InboxClassCompliance, organization.Class)
	require.Equal(t, ptr.To("Europe/Amsterdam"), organization.RecipientTimezone)
}
