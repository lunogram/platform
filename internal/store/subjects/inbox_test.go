package subjects

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/store"
	"github.com/stretchr/testify/require"
)

func containsInboxMessage(messages InboxMessages, id uuid.UUID) bool {
	for i := range messages {
		if messages[i].ID == id {
			return true
		}
	}
	return false
}

// titleContent is a tiny helper so tests can keep reading like "Title: ..."
// while the column is JSONB. Production code builds Content via the renderer.
func titleContent(title string) json.RawMessage {
	body, _ := json.Marshal(map[string]string{"title": title})
	return body
}

func TestCreateUserInboxMessageIdempotency(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	ctx := context.Background()
	projectID := uuid.New()
	userID, err := db.UpsertUser(ctx, projectID, UpsertUserParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "user-1"}},
	})
	require.NoError(t, err)

	external := "retry-1"
	params := InboxMessageParams{
		ExternalID: &external,
		Channel:    "push",
		Content:    titleContent("Welcome"),
		Priority:   ptr.To(int16(3)),
	}

	first, err := db.CreateUserInboxMessage(ctx, projectID, userID, params)
	require.NoError(t, err)
	require.NotNil(t, first)

	retried, err := db.CreateUserInboxMessage(ctx, projectID, userID, params)
	require.NoError(t, err)
	require.Equal(t, first.ID, retried.ID)

	// A second call with different content must still resolve to the same row;
	// idempotency is keyed solely on (project_id, user_id, channel, external_id).
	changed := params
	changed.Content = titleContent("Different")
	retried, err = db.CreateUserInboxMessage(ctx, projectID, userID, changed)
	require.NoError(t, err)
	require.Equal(t, first.ID, retried.ID)
}

func TestUserInboxMessageFiltersAndCounts(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	ctx := context.Background()
	projectID := uuid.New()
	userID, err := db.UpsertUser(ctx, projectID, UpsertUserParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "user-filtered"}},
	})
	require.NoError(t, err)

	unread, err := db.CreateUserInboxMessage(ctx, projectID, userID, InboxMessageParams{Channel: "push", Content: titleContent("Unread")})
	require.NoError(t, err)
	opened, err := db.CreateUserInboxMessage(ctx, projectID, userID, InboxMessageParams{Channel: "push", Content: titleContent("Opened")})
	require.NoError(t, err)
	_, transitioned, err := db.ReadUserInboxMessage(ctx, projectID, userID, opened.ID)
	require.NoError(t, err)
	require.True(t, transitioned)
	archived, err := db.CreateUserInboxMessage(ctx, projectID, userID, InboxMessageParams{Channel: "push", Content: titleContent("Archived")})
	require.NoError(t, err)
	_, transitioned, err = db.ArchiveUserInboxMessage(ctx, projectID, userID, archived.ID)
	require.NoError(t, err)
	require.True(t, transitioned)
	future := time.Now().Add(time.Hour)
	_, err = db.CreateUserInboxMessage(ctx, projectID, userID, InboxMessageParams{Channel: "push", Content: titleContent("Future"), ScheduledAt: &future})
	require.NoError(t, err)
	expiredAt := time.Now().Add(-time.Minute)
	_, err = db.CreateUserInboxMessage(ctx, projectID, userID, InboxMessageParams{Channel: "push", Content: titleContent("Expired"), ExpiresAt: &expiredAt})
	require.NoError(t, err)

	defaultMessages, total, err := db.ListUserInboxMessages(ctx, projectID, userID, store.Pagination{Limit: 10}, InboxListFilter{})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.ElementsMatch(t, []uuid.UUID{unread.ID, opened.ID}, inboxMessageIDs(defaultMessages))

	unreadMessages, total, err := db.ListUserInboxMessages(ctx, projectID, userID, store.Pagination{Limit: 10}, InboxListFilter{Status: InboxStatusUnread})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, []uuid.UUID{unread.ID}, inboxMessageIDs(unreadMessages))

	withArchived, total, err := db.ListUserInboxMessages(ctx, projectID, userID, store.Pagination{Limit: 10}, InboxListFilter{IncludeArchived: true})
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.ElementsMatch(t, []uuid.UUID{unread.ID, opened.ID, archived.ID}, inboxMessageIDs(withArchived))

	counts, err := db.CountUserInboxMessages(ctx, projectID, userID, "push")
	require.NoError(t, err)
	require.Equal(t, InboxCounts{Unread: 1, Total: 2}, counts)
}

func TestUserInboxMessageScheduling(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	ctx := context.Background()
	projectID := uuid.New()
	userID, err := db.UpsertUser(ctx, projectID, UpsertUserParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "user-scheduled"}},
	})
	require.NoError(t, err)

	future := time.Now().Add(time.Hour).Truncate(time.Second)
	message, err := db.CreateUserInboxMessage(ctx, projectID, userID, InboxMessageParams{
		Channel:     "push",
		Content:     titleContent("Later"),
		ScheduledAt: &future,
	})
	require.NoError(t, err)
	require.Equal(t, future, message.ScheduledAt.Truncate(time.Second))

	visible, total, err := db.ListUserInboxMessages(ctx, projectID, userID, store.Pagination{Limit: 10}, InboxListFilter{})
	require.NoError(t, err)
	require.Empty(t, visible)
	require.Zero(t, total)

	all, total, err := db.ListUserInboxMessages(ctx, projectID, userID, store.Pagination{Limit: 10}, InboxListFilter{IncludeScheduled: true})
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.Equal(t, 1, total)

	_, transitioned, err := db.ReadUserInboxMessage(ctx, projectID, userID, message.ID)
	require.NoError(t, err)
	require.False(t, transitioned)

	processed, err := db.ScanDueUserInboxMessages(ctx, 1000, func(message InboxMessage) error {
		require.FailNow(t, "future message should not be due", message.ID.String())
		return nil
	})
	require.NoError(t, err)
	require.Zero(t, processed)

	past := time.Now().Add(-time.Minute).Truncate(time.Second)
	updated, err := db.UpdateUserInboxMessageScheduledAt(ctx, projectID, userID, message.ID, past)
	require.NoError(t, err)
	require.Equal(t, past, updated.ScheduledAt.Truncate(time.Second))

	processed, err = db.ScanDueUserInboxMessages(ctx, 1000, func(due InboxMessage) error {
		require.Equal(t, message.ID, due.ID)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	reopened, transitioned, err := db.ReadUserInboxMessage(ctx, projectID, userID, message.ID)
	require.NoError(t, err)
	require.True(t, transitioned)
	require.NotNil(t, reopened.ReadAt)
}

func TestOrganizationInboxMessageScheduling(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	ctx := context.Background()
	projectID := uuid.New()
	organizationID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "org-scheduled"}},
	})
	require.NoError(t, err)

	future := time.Now().Add(time.Hour).Truncate(time.Second)
	message, err := db.CreateOrganizationInboxMessage(ctx, projectID, organizationID, InboxMessageParams{
		Channel:     "push",
		Content:     titleContent("Later"),
		ScheduledAt: &future,
	})
	require.NoError(t, err)

	visible, total, err := db.ListOrganizationInboxMessages(ctx, projectID, organizationID, store.Pagination{Limit: 10}, InboxListFilter{})
	require.NoError(t, err)
	require.Empty(t, visible)
	require.Zero(t, total)

	_, transitioned, err := db.ArchiveOrganizationInboxMessage(ctx, projectID, organizationID, message.ID)
	require.NoError(t, err)
	require.False(t, transitioned)

	past := time.Now().Add(-time.Minute).Truncate(time.Second)
	_, err = db.UpdateOrganizationInboxMessageScheduledAt(ctx, projectID, organizationID, message.ID, past)
	require.NoError(t, err)

	processed, err := db.ScanDueOrganizationInboxMessages(ctx, 1000, func(due InboxMessage) error {
		require.Equal(t, message.ID, due.ID)
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, processed)
}

func inboxMessageIDs(messages InboxMessages) []uuid.UUID {
	ids := make([]uuid.UUID, len(messages))
	for i := range messages {
		ids[i] = messages[i].ID
	}
	return ids
}

func TestMarkInboxMessageTerminalOutcomes(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	ctx := context.Background()
	projectID := uuid.New()
	userID, err := db.UpsertUser(ctx, projectID, UpsertUserParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "user-terminal"}},
	})
	require.NoError(t, err)

	newMessage := func(title string) uuid.UUID {
		t.Helper()
		message, err := db.CreateUserInboxMessage(ctx, projectID, userID, InboxMessageParams{
			Channel: "push",
			Content: titleContent(title),
		})
		require.NoError(t, err)
		return message.ID
	}

	t.Run("failure is recorded once", func(t *testing.T) {
		id := newMessage("Failing")

		failed, err := db.MarkUserInboxMessageFailed(ctx, id, "suppressed: recipient opted out")
		require.NoError(t, err)
		require.True(t, failed)

		failed, err = db.MarkUserInboxMessageFailed(ctx, id, "a different reason")
		require.NoError(t, err)
		require.False(t, failed)

		message, err := db.GetUserInboxMessageByID(ctx, id)
		require.NoError(t, err)
		require.NotNil(t, message.FailedAt)
		require.NotNil(t, message.FailureReason)
		require.Equal(t, "suppressed: recipient opted out", *message.FailureReason)
	})

	t.Run("a failed message can never be marked sent", func(t *testing.T) {
		id := newMessage("Failed then sent")

		failed, err := db.MarkUserInboxMessageFailed(ctx, id, "permanent provider rejection")
		require.NoError(t, err)
		require.True(t, failed)

		sent, err := db.MarkUserInboxMessageSent(ctx, id)
		require.NoError(t, err)
		require.False(t, sent)
	})

	t.Run("a sent message can never be marked failed", func(t *testing.T) {
		id := newMessage("Sent then failed")

		sent, err := db.MarkUserInboxMessageSent(ctx, id)
		require.NoError(t, err)
		require.True(t, sent)

		failed, err := db.MarkUserInboxMessageFailed(ctx, id, "too late")
		require.NoError(t, err)
		require.False(t, failed)
	})

	t.Run("a failed message drops out of the due scan", func(t *testing.T) {
		id := newMessage("Not rescanned")

		failed, err := db.MarkUserInboxMessageFailed(ctx, id, "permanent provider rejection")
		require.NoError(t, err)
		require.True(t, failed)

		var scanned []uuid.UUID
		_, err = db.ScanDueUserInboxMessages(ctx, 100, func(m InboxMessage) error {
			scanned = append(scanned, m.ID)
			return nil
		})
		require.NoError(t, err)
		require.NotContains(t, scanned, id)
	})

	t.Run("class defaults to standard", func(t *testing.T) {
		id := newMessage("Classified")

		message, err := db.GetUserInboxMessageByID(ctx, id)
		require.NoError(t, err)
		require.Equal(t, InboxClassStandard, message.Class)
		require.Nil(t, message.RecipientTimezone)
	})

	t.Run("a failed message is hidden only when the filter asks", func(t *testing.T) {
		failedID := newMessage("Suppressed")
		liveID := newMessage("Delivered")

		failed, err := db.MarkUserInboxMessageFailed(ctx, failedID, "suppressed: recipient opted out")
		require.NoError(t, err)
		require.True(t, failed)

		pagination := store.Pagination{Limit: 100}

		console, _, err := db.ListUserInboxMessages(ctx, projectID, userID, pagination, InboxListFilter{})
		require.NoError(t, err)
		require.True(t, containsInboxMessage(console, failedID), "the console must still see a failed message")
		require.True(t, containsInboxMessage(console, liveID))

		enduser, _, err := db.ListUserInboxMessages(ctx, projectID, userID, pagination, InboxListFilter{ExcludeFailed: true})
		require.NoError(t, err)
		require.False(t, containsInboxMessage(enduser, failedID), "the end user must not see a failed message")
		require.True(t, containsInboxMessage(enduser, liveID), "filtering must not hide messages that did not fail")
	})

	t.Run("organization messages settle the same way", func(t *testing.T) {
		organizationID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
			Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "org-terminal"}},
		})
		require.NoError(t, err)

		message, err := db.CreateOrganizationInboxMessage(ctx, projectID, organizationID, InboxMessageParams{
			Channel: "push",
			Content: titleContent("Org failing"),
		})
		require.NoError(t, err)

		failed, err := db.MarkOrganizationInboxMessageFailed(ctx, message.ID, "permanent provider rejection")
		require.NoError(t, err)
		require.True(t, failed)

		failed, err = db.MarkOrganizationInboxMessageFailed(ctx, message.ID, "again")
		require.NoError(t, err)
		require.False(t, failed)

		sent, err := db.MarkOrganizationInboxMessageSent(ctx, message.ID)
		require.NoError(t, err)
		require.False(t, sent)
	})
}
