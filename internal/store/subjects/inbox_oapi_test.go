package subjects

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/stretchr/testify/require"
)

// TestInboxMessageOAPIMapping is a pure (non-DB) test that the InboxMessage ->
// oapi.InboxMessage projection carries every field through, in particular the
// optional timestamps (sent_at was previously dropped on the floor).
func TestInboxMessageOAPIMapping(t *testing.T) {
	t.Parallel()

	id := uuid.New()
	projectID := uuid.New()
	userID := uuid.New()
	sentAt := time.Date(2026, 6, 24, 10, 30, 0, 0, time.UTC)
	readAt := time.Date(2026, 6, 24, 11, 0, 0, 0, time.UTC)
	scheduledAt := time.Date(2026, 6, 24, 9, 0, 0, 0, time.UTC)

	m := InboxMessage{
		ID:          id,
		ProjectID:   projectID,
		UserID:      &userID,
		Channel:     "email",
		Content:     json.RawMessage(`{"title":"Hi"}`),
		Data:        json.RawMessage(`{}`),
		Tags:        []string{"welcome"},
		Priority:    DefaultInboxPriority,
		ScheduledAt: scheduledAt,
		ReadAt:      &readAt,
		SentAt:      &sentAt,
		CreatedAt:   scheduledAt,
		UpdatedAt:   scheduledAt,
	}

	out := m.OAPI()

	require.Equal(t, id, out.Id)
	require.Equal(t, projectID, out.ProjectId)
	require.Equal(t, &userID, out.UserId)
	require.Equal(t, []string{"welcome"}, out.Tags)
	require.Equal(t, DefaultInboxPriority, out.Priority)
	require.Equal(t, scheduledAt, out.ScheduledAt)
	require.Equal(t, &readAt, out.ReadAt)
	// The field this PR added — must survive the projection.
	require.Equal(t, &sentAt, out.SentAt)
}

// TestInboxMessagesOAPIMapping covers the slice projection.
func TestInboxMessagesOAPIMapping(t *testing.T) {
	t.Parallel()

	sentAt := time.Date(2026, 6, 24, 10, 30, 0, 0, time.UTC)
	ms := InboxMessages{
		{ID: uuid.New(), Channel: "push", SentAt: &sentAt},
		{ID: uuid.New(), Channel: "sms"},
	}

	out := ms.OAPI()

	require.Len(t, out, 2)
	require.Equal(t, &sentAt, out[0].SentAt)
	require.Nil(t, out[1].SentAt)
}

// TestNormalizeInboxMessageParamsDefaults verifies the defaults applied to a
// bare params struct, including the default priority constant.
func TestNormalizeInboxMessageParamsDefaults(t *testing.T) {
	t.Parallel()

	got := normalizeInboxMessageParams(InboxMessageParams{})

	require.JSONEq(t, `{}`, string(got.Content))
	require.JSONEq(t, `{}`, string(got.Data))
	require.NotNil(t, got.Tags)
	require.Empty(t, got.Tags)
	require.NotNil(t, got.Priority)
	require.Equal(t, DefaultInboxPriority, *got.Priority)
}

// TestNormalizeInboxMessageParamsPreservesExplicitPriority ensures an explicit
// priority is left untouched.
func TestNormalizeInboxMessageParamsPreservesExplicitPriority(t *testing.T) {
	t.Parallel()

	got := normalizeInboxMessageParams(InboxMessageParams{Priority: ptr.To(int16(5))})
	require.Equal(t, int16(5), *got.Priority)
}
