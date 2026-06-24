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
