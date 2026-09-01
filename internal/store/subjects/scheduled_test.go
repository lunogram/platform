package subjects

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/store"
	"github.com/stretchr/testify/require"
)

func TestUpsertSchedule(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create a single schedule
	id, err := db.UpsertSchedule(ctx, projectID, "birthday", "single")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id)

	// Retrieve it by name
	schedule, err := db.GetScheduleByName(ctx, projectID, "birthday")
	require.NoError(t, err)
	require.Equal(t, id, schedule.ID)
	require.Equal(t, projectID, schedule.ProjectID)
	require.Equal(t, "birthday", schedule.Name)
	require.Equal(t, "single", schedule.Type)

	// Retrieve it by ID
	schedule2, err := db.GetScheduleByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, id, schedule2.ID)

	// Upsert again – should return the same ID and update the type
	id2, err := db.UpsertSchedule(ctx, projectID, "birthday", "recurring")
	require.NoError(t, err)
	require.Equal(t, id, id2)

	updated, err := db.GetScheduleByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "recurring", updated.Type)
}

func TestUpsertScheduleCreatesDefaultOffset(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	id, err := db.UpsertSchedule(ctx, projectID, "renewal", "single")
	require.NoError(t, err)

	offsets, err := db.ListScheduleOffsets(ctx, id)
	require.NoError(t, err)
	require.Len(t, offsets, 1)
	require.Equal(t, "after", offsets[0].Direction)
	// The default offset should represent 0 minutes
	require.Contains(t, offsets[0].Offset, "00:00:00")
}

func TestDeleteSchedule(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	id, err := db.UpsertSchedule(ctx, projectID, "trial_end", "single")
	require.NoError(t, err)

	err = db.DeleteSchedule(ctx, projectID, "trial_end")
	require.NoError(t, err)

	_, err = db.GetScheduleByName(ctx, projectID, "trial_end")
	require.Error(t, err, "should not find soft-deleted schedule by name")

	_, err = db.GetScheduleByID(ctx, id)
	require.Error(t, err, "should not find soft-deleted schedule by ID")

	// Deleting again should return ErrNoRows
	err = db.DeleteSchedule(ctx, projectID, "trial_end")
	require.ErrorIs(t, err, store.ErrNoRows)
}

func TestDeleteScheduleByID(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	id, err := db.UpsertSchedule(ctx, projectID, "contract_end", "single")
	require.NoError(t, err)

	err = db.DeleteScheduleByID(ctx, projectID, id)
	require.NoError(t, err)

	_, err = db.GetScheduleByID(ctx, id)
	require.Error(t, err)

	// Deleting again
	err = db.DeleteScheduleByID(ctx, projectID, id)
	require.ErrorIs(t, err, store.ErrNoRows)
}

func TestDeleteScheduleNonExistent(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	err := db.DeleteSchedule(ctx, projectID, "does_not_exist")
	require.ErrorIs(t, err, store.ErrNoRows)
}

func TestListSchedules(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	_, err := db.UpsertSchedule(ctx, projectID, "alpha", "single")
	require.NoError(t, err)
	_, err = db.UpsertSchedule(ctx, projectID, "beta", "recurring")
	require.NoError(t, err)

	schedules, err := db.ListSchedules(ctx, projectID)
	require.NoError(t, err)
	require.Len(t, schedules, 2)

	names := make(map[string]bool)
	for _, s := range schedules {
		names[s.Name] = true
	}
	require.True(t, names["alpha"])
	require.True(t, names["beta"])
}

func TestListSchedulesEmpty(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	ctx := context.Background()

	schedules, err := db.ListSchedules(ctx, uuid.New())
	require.NoError(t, err)
	require.Empty(t, schedules)
}

func TestListSchedulesExcludesDeleted(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	_, err := db.UpsertSchedule(ctx, projectID, "keep", "single")
	require.NoError(t, err)
	_, err = db.UpsertSchedule(ctx, projectID, "remove", "single")
	require.NoError(t, err)

	err = db.DeleteSchedule(ctx, projectID, "remove")
	require.NoError(t, err)

	schedules, err := db.ListSchedules(ctx, projectID)
	require.NoError(t, err)
	require.Len(t, schedules, 1)
	require.Equal(t, "keep", schedules[0].Name)
}

func TestUpsertScheduleRecoversDeleted(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	id1, err := db.UpsertSchedule(ctx, projectID, "reborn", "single")
	require.NoError(t, err)

	err = db.DeleteSchedule(ctx, projectID, "reborn")
	require.NoError(t, err)

	id2, err := db.UpsertSchedule(ctx, projectID, "reborn", "recurring")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id2)

	// The partial unique index (WHERE deleted_at IS NULL) means a soft-deleted
	// row does not conflict, so a brand-new row is created with a new ID.
	require.NotEqual(t, id1, id2, "soft-deleted row does not conflict; a new row is created")

	schedule, err := db.GetScheduleByName(ctx, projectID, "reborn")
	require.NoError(t, err)
	require.Equal(t, "recurring", schedule.Type)
	require.Equal(t, id2, schedule.ID)
}

func TestCreateScheduleOffset(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	scheduleID, err := db.UpsertSchedule(ctx, projectID, "appointment", "single")
	require.NoError(t, err)

	offset, err := db.CreateScheduleOffset(ctx, scheduleID, "30 minutes", "before")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, offset.ID)
	require.Equal(t, scheduleID, offset.ScheduleID)
	require.Equal(t, "before", offset.Direction)

	// Retrieve it
	got, err := db.GetScheduleOffset(ctx, offset.ID)
	require.NoError(t, err)
	require.Equal(t, offset.ID, got.ID)
	require.Equal(t, "before", got.Direction)
}

func TestCreateScheduleOffsetInvalidDirection(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	scheduleID, err := db.UpsertSchedule(ctx, projectID, "test_dir", "single")
	require.NoError(t, err)

	_, err = db.CreateScheduleOffset(ctx, scheduleID, "10 minutes", "sideways")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid direction")
}

func TestListScheduleOffsets(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	scheduleID, err := db.UpsertSchedule(ctx, projectID, "event_schedule", "single")
	require.NoError(t, err)

	_, err = db.CreateScheduleOffset(ctx, scheduleID, "1 hour", "before")
	require.NoError(t, err)
	_, err = db.CreateScheduleOffset(ctx, scheduleID, "1 day", "after")
	require.NoError(t, err)

	offsets, err := db.ListScheduleOffsets(ctx, scheduleID)
	require.NoError(t, err)
	// Default offset + 2 custom = 3
	require.Len(t, offsets, 3)
}

func TestDeleteScheduleOffset(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	scheduleID, err := db.UpsertSchedule(ctx, projectID, "del_offset", "single")
	require.NoError(t, err)

	offset, err := db.CreateScheduleOffset(ctx, scheduleID, "2 hours", "before")
	require.NoError(t, err)

	err = db.DeleteScheduleOffset(ctx, offset.ID)
	require.NoError(t, err)

	_, err = db.GetScheduleOffset(ctx, offset.ID)
	require.Error(t, err)
}

func TestDeleteScheduleOffsetCannotDeleteDefault(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	scheduleID, err := db.UpsertSchedule(ctx, projectID, "no_del_default", "single")
	require.NoError(t, err)

	offsets, err := db.ListScheduleOffsets(ctx, scheduleID)
	require.NoError(t, err)
	require.Len(t, offsets, 1)

	// Trying to delete the default offset should fail with ErrNoRows
	err = db.DeleteScheduleOffset(ctx, offsets[0].ID)
	require.ErrorIs(t, err, store.ErrNoRows)
}

func TestUpsertAndListScheduledSchemas(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	scheduleID, err := db.UpsertSchedule(ctx, projectID, "checkout", "single")
	require.NoError(t, err)

	paths := rules.Paths{
		{Path: ".amount", Type: rules.TypeNumber},
		{Path: ".currency", Type: rules.TypeString},
	}

	err = db.UpsertScheduledSchema(ctx, projectID, scheduleID, paths)
	require.NoError(t, err)

	schemas, err := db.ListScheduledSchemas(ctx, projectID, "checkout")
	require.NoError(t, err)
	require.Len(t, schemas, 2)

	schemaMap := make(map[string][]string)
	for _, s := range schemas {
		schemaMap[s.Path] = s.Types
	}
	require.Contains(t, schemaMap, ".amount")
	require.Contains(t, schemaMap, ".currency")
}

func TestUpsertScheduledSchemaIdempotent(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	scheduleID, err := db.UpsertSchedule(ctx, projectID, "schema_idem", "single")
	require.NoError(t, err)

	paths := rules.Paths{{Path: ".key", Type: rules.TypeString}}

	err = db.UpsertScheduledSchema(ctx, projectID, scheduleID, paths)
	require.NoError(t, err)

	// Insert the same again – should not error
	err = db.UpsertScheduledSchema(ctx, projectID, scheduleID, paths)
	require.NoError(t, err)

	schemas, err := db.ListScheduledSchemas(ctx, projectID, "schema_idem")
	require.NoError(t, err)
	require.Len(t, schemas, 1)
}

func TestListSchedulesWithSchemas(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	scheduleID, err := db.UpsertSchedule(ctx, projectID, "with_schema", "single")
	require.NoError(t, err)

	paths := rules.Paths{
		{Path: ".foo", Type: rules.TypeString},
		{Path: ".bar", Type: rules.TypeNumber},
	}
	err = db.UpsertScheduledSchema(ctx, projectID, scheduleID, paths)
	require.NoError(t, err)

	schedules, err := db.ListSchedules(ctx, projectID)
	require.NoError(t, err)
	require.Len(t, schedules, 1)
	require.Len(t, schedules[0].Schema, 2)
}

func createTestUserForSchedules(t *testing.T, db *State, ctx context.Context, projectID uuid.UUID) uuid.UUID {
	t.Helper()
	anonID := uuid.New().String()
	userID, err := db.CreateUser(ctx, projectID, nil, nil, json.RawMessage(`{}`), nil, nil, []ExternalIDParam{{Source: "anonymous", ExternalID: anonID}})
	require.NoError(t, err)
	return userID
}

func createTestOrgForSchedules(t *testing.T, db *State, ctx context.Context, projectID uuid.UUID) uuid.UUID {
	t.Helper()
	orgID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: uuid.New().String()}},
		Name:        ptr.To("Test Org"),
	})
	require.NoError(t, err)
	return orgID
}

func TestCreateUserScheduleSingle(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "birthday", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	data := json.RawMessage(`{"note":"happy birthday"}`)

	us, err := db.CreateUserSchedule(ctx, userID, scheduleID, &futureTime, nil, nil, data)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, us.ID)
	require.Equal(t, userID, us.UserID)
	require.Equal(t, scheduleID, us.ScheduleID)
	require.NotNil(t, us.ScheduledAt)
	require.WithinDuration(t, futureTime, *us.ScheduledAt, time.Second)

	// Should have generated a scheduled event for the default offset
	events, err := db.ListPendingScheduledEventsForUser(ctx, userID, scheduleID)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.WithinDuration(t, futureTime, events[0].FireAt, time.Second)
}

func TestCreateUserScheduleWithMultipleOffsets(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "renewal", "single")
	require.NoError(t, err)

	// Add extra offsets
	_, err = db.CreateScheduleOffset(ctx, scheduleID, "1 day", "before")
	require.NoError(t, err)
	_, err = db.CreateScheduleOffset(ctx, scheduleID, "1 hour", "after")
	require.NoError(t, err)

	futureTime := time.Now().Add(7 * 24 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = db.CreateUserSchedule(ctx, userID, scheduleID, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	events, err := db.ListPendingScheduledEventsForUser(ctx, userID, scheduleID)
	require.NoError(t, err)
	// Default + 1 day before + 1 hour after = 3 events
	require.Len(t, events, 3)
}

func TestGetUserScheduleByID(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "get_test", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Microsecond)
	us, err := db.CreateUserSchedule(ctx, userID, scheduleID, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	got, err := db.GetUserScheduleByID(ctx, us.ID)
	require.NoError(t, err)
	require.Equal(t, us.ID, got.ID)
	require.Equal(t, us.UserID, got.UserID)
}

func TestDeleteUserSchedule(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "to_delete", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	us, err := db.CreateUserSchedule(ctx, userID, scheduleID, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	err = db.DeleteUserSchedule(ctx, us.ID)
	require.NoError(t, err)

	_, err = db.GetUserScheduleByID(ctx, us.ID)
	require.Error(t, err)

	// Events should be cascade-deleted
	events, err := db.ListPendingScheduledEventsForUser(ctx, userID, scheduleID)
	require.NoError(t, err)
	require.Empty(t, events)
}

func TestDeleteUserScheduleByScheduleID(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "del_by_sid", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = db.CreateUserSchedule(ctx, userID, scheduleID, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	err = db.DeleteUserScheduleByScheduleID(ctx, userID, scheduleID)
	require.NoError(t, err)

	results, err := db.ListUserSchedulesByScheduleID(ctx, userID, scheduleID)
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestUpdateUserSchedule(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "updatable", "single")
	require.NoError(t, err)

	original := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	us, err := db.CreateUserSchedule(ctx, userID, scheduleID, &original, nil, nil, json.RawMessage(`{"v":1}`))
	require.NoError(t, err)

	newTime := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Microsecond)
	newData := json.RawMessage(`{"v":2}`)
	updated, err := db.UpdateUserSchedule(ctx, us.ID, &newTime, nil, nil, newData)
	require.NoError(t, err)
	require.WithinDuration(t, newTime, *updated.ScheduledAt, time.Second)

	// Events should have been regenerated
	events, err := db.ListPendingScheduledEventsForUser(ctx, userID, scheduleID)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.WithinDuration(t, newTime, events[0].FireAt, time.Second)
}

func TestUpdateUserSchedulePreservesNilFields(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "preserve_nil", "single")
	require.NoError(t, err)

	original := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Microsecond)
	originalData := json.RawMessage(`{"key":"value"}`)
	us, err := db.CreateUserSchedule(ctx, userID, scheduleID, &original, nil, nil, originalData)
	require.NoError(t, err)

	// Update with nil data – should preserve existing data
	updated, err := db.UpdateUserSchedule(ctx, us.ID, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, updated.ScheduledAt)
	require.WithinDuration(t, original, *updated.ScheduledAt, time.Second)
	require.JSONEq(t, `{"key":"value"}`, string(updated.Data))
}

func TestUpsertUserSchedule(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "upsert_test", "single")
	require.NoError(t, err)

	time1 := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	us1, err := db.UpsertUserSchedule(ctx, uuid.Nil, userID, scheduleID, &time1, nil, nil, json.RawMessage(`{"v":1}`))
	require.NoError(t, err)

	// Re-upserting with the same assignment id updates that row in place.
	time2 := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Microsecond)
	us2, err := db.UpsertUserSchedule(ctx, us1.ID, userID, scheduleID, &time2, nil, nil, json.RawMessage(`{"v":2}`))
	require.NoError(t, err)

	require.Equal(t, us1.ID, us2.ID)
	require.WithinDuration(t, time2, *us2.ScheduledAt, time.Second)
}

func TestUpsertUserScheduleCreatesDistinctInstances(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "multi_instance", "single")
	require.NoError(t, err)

	// Two upserts without an id create two distinct assignments for the same
	// (user, schedule) — the same name can now be scheduled multiple times.
	time1 := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	us1, err := db.UpsertUserSchedule(ctx, uuid.Nil, userID, scheduleID, &time1, nil, nil, json.RawMessage(`{"v":1}`))
	require.NoError(t, err)

	time2 := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Microsecond)
	us2, err := db.UpsertUserSchedule(ctx, uuid.Nil, userID, scheduleID, &time2, nil, nil, json.RawMessage(`{"v":2}`))
	require.NoError(t, err)

	require.NotEqual(t, us1.ID, us2.ID)

	all, err := db.ListUserSchedulesByScheduleID(ctx, userID, scheduleID)
	require.NoError(t, err)
	require.Len(t, all, 2)
}

func TestUpsertUserScheduleOwnershipMismatch(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userA := createTestUserForSchedules(t, db, ctx, projectID)
	userB := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "ownership", "single")
	require.NoError(t, err)

	at := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	owned, err := db.UpsertUserSchedule(ctx, uuid.Nil, userA, scheduleID, &at, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	// Re-using userA's assignment id under a different user must be rejected
	// rather than silently rewriting userA's row.
	_, err = db.UpsertUserSchedule(ctx, owned.ID, userB, scheduleID, &at, nil, nil, json.RawMessage(`{}`))
	require.ErrorIs(t, err, ErrScheduleOwnershipMismatch)
}

func TestListUserSchedules(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)

	for i := 0; i < 5; i++ {
		name := "list_sched_" + uuid.New().String()[:8]
		sid, err := db.UpsertSchedule(ctx, projectID, name, "single")
		require.NoError(t, err)

		ft := time.Now().Add(time.Duration(i+1) * 24 * time.Hour).UTC().Truncate(time.Microsecond)
		_, err = db.CreateUserSchedule(ctx, userID, sid, &ft, nil, nil, json.RawMessage(`{}`))
		require.NoError(t, err)
	}

	items, total, err := db.ListUserSchedules(ctx, projectID, userID, store.Pagination{Limit: 3, Offset: 0})
	require.NoError(t, err)
	require.Len(t, items, 3)
	require.Equal(t, 5, total)

	items2, total2, err := db.ListUserSchedules(ctx, projectID, userID, store.Pagination{Limit: 3, Offset: 3})
	require.NoError(t, err)
	require.Len(t, items2, 2)
	require.Equal(t, 5, total2)
}

func TestListUserSchedulesHasPendingEvents(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "pending_test", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = db.CreateUserSchedule(ctx, userID, scheduleID, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	// An unfired event whose fire_at is still in the future is NOT "sending":
	// has_pending_events only reflects events that are actually due. (Regression:
	// recurring schedules pre-generate the next occurrence's event, which kept the
	// "Sending" badge lit indefinitely.)
	items, _, err := db.ListUserSchedules(ctx, projectID, userID, store.Pagination{Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.False(t, items[0].HasPendingEvents)

	events, err := db.ListPendingScheduledEventsForUser(ctx, userID, scheduleID)
	require.NoError(t, err)
	require.Len(t, events, 1)

	// Once the event is due (fire_at <= now), the schedule reports pending events.
	pastTime := time.Now().Add(-1 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = db.ScheduledStore.db.ExecContext(ctx,
		`UPDATE user_scheduled_events SET fire_at = $1 WHERE id = $2`,
		pastTime, events[0].ID)
	require.NoError(t, err)

	items, _, err = db.ListUserSchedules(ctx, projectID, userID, store.Pagination{Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.True(t, items[0].HasPendingEvents)

	// Firing the event clears the flag.
	require.NoError(t, db.MarkScheduledEventFired(ctx, events[0].ID))

	items, _, err = db.ListUserSchedules(ctx, projectID, userID, store.Pagination{Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.False(t, items[0].HasPendingEvents)
}

func TestListUserSchedulesEmpty(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)

	items, total, err := db.ListUserSchedules(ctx, projectID, userID, store.Pagination{Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Empty(t, items)
	require.Equal(t, 0, total)
}

func TestListUserSchedulesByScheduleID(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "by_sid", "single")
	require.NoError(t, err)

	ft := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = db.CreateUserSchedule(ctx, userID, scheduleID, &ft, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	results, err := db.ListUserSchedulesByScheduleID(ctx, userID, scheduleID)
	require.NoError(t, err)
	require.Len(t, results, 1)
}

func TestListDueScheduledEvents(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "due_test", "single")
	require.NoError(t, err)

	// Schedule in the past so the event is immediately due
	pastTime := time.Now().Add(-1 * time.Hour).UTC().Truncate(time.Microsecond)

	// We need to insert directly since generateScheduledEvents filters out past fire_at.
	// Instead, create with a future time, then manually update fire_at.
	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = db.CreateUserSchedule(ctx, userID, scheduleID, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	events, err := db.ListPendingScheduledEventsForUser(ctx, userID, scheduleID)
	require.NoError(t, err)
	require.Len(t, events, 1)

	// Manually set fire_at to past so the event becomes due
	_, err = db.ScheduledStore.db.ExecContext(ctx,
		`UPDATE user_scheduled_events SET fire_at = $1 WHERE id = $2`,
		pastTime, events[0].ID)
	require.NoError(t, err)

	dueEvents, err := db.ListDueScheduledEvents(ctx, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(dueEvents), 1)

	found := false
	for _, e := range dueEvents {
		if e.ID == events[0].ID {
			found = true
			require.Equal(t, "due_test", e.ScheduleName)
			require.Equal(t, projectID, e.ProjectID)
		}
	}
	require.True(t, found, "expected to find the due event")
}

func TestMarkScheduledEventFired(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "fire_test", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = db.CreateUserSchedule(ctx, userID, scheduleID, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	events, err := db.ListPendingScheduledEventsForUser(ctx, userID, scheduleID)
	require.NoError(t, err)
	require.Len(t, events, 1)

	err = db.MarkScheduledEventFired(ctx, events[0].ID)
	require.NoError(t, err)

	// After marking fired, pending list should be empty
	pending, err := db.ListPendingScheduledEventsForUser(ctx, userID, scheduleID)
	require.NoError(t, err)
	require.Empty(t, pending)
}

func TestScanDueScheduledEvents(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "scan_due", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = db.CreateUserSchedule(ctx, userID, scheduleID, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	events, err := db.ListPendingScheduledEventsForUser(ctx, userID, scheduleID)
	require.NoError(t, err)
	require.Len(t, events, 1)

	// Move fire_at to past
	pastTime := time.Now().Add(-1 * time.Hour).UTC()
	_, err = db.ScheduledStore.db.ExecContext(ctx,
		`UPDATE user_scheduled_events SET fire_at = $1 WHERE id = $2`,
		pastTime, events[0].ID)
	require.NoError(t, err)

	var scanned []DueScheduledEvent
	_, err = db.ScanDueScheduledEvents(ctx, 1000, func(e DueScheduledEvent) error {
		scanned = append(scanned, e)
		return nil
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(scanned), 1)
}

func TestCreateUserScheduleRecurring(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "monthly_report", "recurring")
	require.NoError(t, err)

	startAt := time.Now().Add(-60 * 24 * time.Hour).UTC().Truncate(time.Microsecond) // 60 days ago
	interval := "1 month"

	us, err := db.CreateUserSchedule(ctx, userID, scheduleID, nil, &startAt, &interval, json.RawMessage(`{}`))
	require.NoError(t, err)

	require.NotNil(t, us.ScheduledAt, "recurring schedule should compute scheduled_at")
	require.True(t, us.ScheduledAt.After(time.Now()), "scheduled_at should be in the future")
	require.Greater(t, us.Occurrence, 0, "occurrence should be > 0 for recurring from past start")
	require.NotNil(t, us.Interval)
	require.Equal(t, "1 mon", *us.Interval)
}

func TestAdvanceAndGenerateUserScheduleEvents(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "advance_test", "recurring")
	require.NoError(t, err)

	startAt := time.Now().Add(-30 * 24 * time.Hour).UTC().Truncate(time.Microsecond) // 30 days ago
	interval := "7 days"

	us, err := db.CreateUserSchedule(ctx, userID, scheduleID, nil, &startAt, &interval, json.RawMessage(`{}`))
	require.NoError(t, err)
	require.NotNil(t, us.ScheduledAt)

	originalOccurrence := us.Occurrence
	originalScheduledAt := *us.ScheduledAt

	// Delete all pending events to simulate a completed cycle
	_, err = db.ScheduledStore.db.ExecContext(ctx,
		`DELETE FROM user_scheduled_events WHERE user_schedule_id = $1`, us.ID)
	require.NoError(t, err)

	// Move scheduled_at to the past so the advance logic sees it
	_, err = db.ScheduledStore.db.ExecContext(ctx,
		`UPDATE user_schedules SET scheduled_at = $1 WHERE id = $2`,
		time.Now().Add(-1*time.Hour).UTC(), us.ID)
	require.NoError(t, err)

	// Re-fetch
	us, err = db.GetUserScheduleByID(ctx, us.ID)
	require.NoError(t, err)

	err = db.AdvanceAndGenerateUserScheduleEvents(ctx, *us)
	require.NoError(t, err)

	// Re-fetch to check advanced state
	advanced, err := db.GetUserScheduleByID(ctx, us.ID)
	require.NoError(t, err)
	require.Greater(t, advanced.Occurrence, originalOccurrence, "occurrence should have increased")
	require.True(t, advanced.ScheduledAt.After(originalScheduledAt), "scheduled_at should have moved forward")

	// Should have new pending events
	newEvents, err := db.ListPendingScheduledEventsForUser(ctx, userID, scheduleID)
	require.NoError(t, err)
	require.NotEmpty(t, newEvents)
}

func TestScanRecurringUserSchedulesWithoutPendingEvents(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "scan_recurring", "recurring")
	require.NoError(t, err)

	startAt := time.Now().Add(-30 * 24 * time.Hour).UTC().Truncate(time.Microsecond)
	interval := "7 days"

	us, err := db.CreateUserSchedule(ctx, userID, scheduleID, nil, &startAt, &interval, json.RawMessage(`{}`))
	require.NoError(t, err)

	// Mark all pending events as fired
	events, err := db.ListPendingScheduledEventsForUser(ctx, userID, scheduleID)
	require.NoError(t, err)
	for _, e := range events {
		require.NoError(t, db.MarkScheduledEventFired(ctx, e.ID))
	}

	// Move scheduled_at to the past
	_, err = db.ScheduledStore.db.ExecContext(ctx,
		`UPDATE user_schedules SET scheduled_at = $1 WHERE id = $2`,
		time.Now().Add(-1*time.Hour).UTC(), us.ID)
	require.NoError(t, err)

	var found []UserSchedule
	_, err = db.ScanRecurringUserSchedulesWithoutPendingEvents(ctx, 1000, func(us UserSchedule) error {
		found = append(found, us)
		return nil
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(found), 1)

	foundOurs := false
	for _, f := range found {
		if f.ID == us.ID {
			foundOurs = true
		}
	}
	require.True(t, foundOurs, "should find our recurring schedule without pending events")
}

func TestBackfillUserScheduledEventsForOffset(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "backfill_user", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(7 * 24 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = db.CreateUserSchedule(ctx, userID, scheduleID, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	// Verify we have 1 event (for the default offset)
	events, err := db.ListPendingScheduledEventsForUser(ctx, userID, scheduleID)
	require.NoError(t, err)
	require.Len(t, events, 1)

	// Add a new offset
	newOffset, err := db.CreateScheduleOffset(ctx, scheduleID, "2 hours", "before")
	require.NoError(t, err)

	// Backfill
	count, err := db.BackfillUserScheduledEventsForOffset(ctx, scheduleID, newOffset.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	// Should now have 2 events
	events, err = db.ListPendingScheduledEventsForUser(ctx, userID, scheduleID)
	require.NoError(t, err)
	require.Len(t, events, 2)
}

func TestCreateOrganizationScheduleSingle(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID := createTestOrgForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "org_birthday", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	data := json.RawMessage(`{"team":"engineering"}`)

	os, err := db.CreateOrganizationSchedule(ctx, orgID, scheduleID, &futureTime, nil, nil, data)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, os.ID)
	require.Equal(t, orgID, os.OrganizationID)
	require.Equal(t, scheduleID, os.ScheduleID)
	require.NotNil(t, os.ScheduledAt)
	require.WithinDuration(t, futureTime, *os.ScheduledAt, time.Second)

	// Should have generated an org scheduled event for the default offset
	events, err := db.ListPendingOrgScheduledEventsForOrg(ctx, orgID, scheduleID)
	require.NoError(t, err)
	require.Len(t, events, 1)
}

func TestGetOrganizationScheduleByID(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID := createTestOrgForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "org_get", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Microsecond)
	os, err := db.CreateOrganizationSchedule(ctx, orgID, scheduleID, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	got, err := db.GetOrganizationScheduleByID(ctx, os.ID)
	require.NoError(t, err)
	require.Equal(t, os.ID, got.ID)
	require.Equal(t, orgID, got.OrganizationID)
}

func TestDeleteOrganizationSchedule(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID := createTestOrgForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "org_del", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	os, err := db.CreateOrganizationSchedule(ctx, orgID, scheduleID, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	err = db.DeleteOrganizationSchedule(ctx, os.ID)
	require.NoError(t, err)

	_, err = db.GetOrganizationScheduleByID(ctx, os.ID)
	require.Error(t, err)

	// Events should be cascade-deleted
	events, err := db.ListPendingOrgScheduledEventsForOrg(ctx, orgID, scheduleID)
	require.NoError(t, err)
	require.Empty(t, events)
}

func TestDeleteOrganizationScheduleByScheduleID(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID := createTestOrgForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "org_del_sid", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = db.CreateOrganizationSchedule(ctx, orgID, scheduleID, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	err = db.DeleteOrganizationScheduleByScheduleID(ctx, orgID, scheduleID)
	require.NoError(t, err)

	results, err := db.ListOrganizationSchedulesByScheduleID(ctx, orgID, scheduleID)
	require.NoError(t, err)
	require.Empty(t, results)
}

func TestUpdateOrganizationSchedule(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID := createTestOrgForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "org_update", "single")
	require.NoError(t, err)

	original := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	os, err := db.CreateOrganizationSchedule(ctx, orgID, scheduleID, &original, nil, nil, json.RawMessage(`{"v":1}`))
	require.NoError(t, err)

	newTime := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Microsecond)
	newData := json.RawMessage(`{"v":2}`)
	updated, err := db.UpdateOrganizationSchedule(ctx, os.ID, &newTime, nil, nil, newData)
	require.NoError(t, err)
	require.WithinDuration(t, newTime, *updated.ScheduledAt, time.Second)

	events, err := db.ListPendingOrgScheduledEventsForOrg(ctx, orgID, scheduleID)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.WithinDuration(t, newTime, events[0].FireAt, time.Second)
}

func TestUpdateOrganizationSchedulePreservesNilFields(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID := createTestOrgForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "org_preserve", "single")
	require.NoError(t, err)

	original := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Microsecond)
	originalData := json.RawMessage(`{"org":"data"}`)
	os, err := db.CreateOrganizationSchedule(ctx, orgID, scheduleID, &original, nil, nil, originalData)
	require.NoError(t, err)

	updated, err := db.UpdateOrganizationSchedule(ctx, os.ID, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, updated.ScheduledAt)
	require.WithinDuration(t, original, *updated.ScheduledAt, time.Second)
	require.JSONEq(t, `{"org":"data"}`, string(updated.Data))
}

func TestUpsertOrganizationSchedule(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID := createTestOrgForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "org_upsert", "single")
	require.NoError(t, err)

	time1 := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	os1, err := db.UpsertOrganizationSchedule(ctx, uuid.Nil, orgID, scheduleID, &time1, nil, nil, json.RawMessage(`{"v":1}`))
	require.NoError(t, err)

	// Re-upserting with the same assignment id updates that row in place.
	time2 := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Microsecond)
	os2, err := db.UpsertOrganizationSchedule(ctx, os1.ID, orgID, scheduleID, &time2, nil, nil, json.RawMessage(`{"v":2}`))
	require.NoError(t, err)

	require.Equal(t, os1.ID, os2.ID)
	require.WithinDuration(t, time2, *os2.ScheduledAt, time.Second)
}

func TestListOrganizationSchedules(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID := createTestOrgForSchedules(t, db, ctx, projectID)

	for i := 0; i < 5; i++ {
		name := "org_list_" + uuid.New().String()[:8]
		sid, err := db.UpsertSchedule(ctx, projectID, name, "single")
		require.NoError(t, err)

		ft := time.Now().Add(time.Duration(i+1) * 24 * time.Hour).UTC().Truncate(time.Microsecond)
		_, err = db.CreateOrganizationSchedule(ctx, orgID, sid, &ft, nil, nil, json.RawMessage(`{}`))
		require.NoError(t, err)
	}

	items, total, err := db.ListOrganizationSchedules(ctx, projectID, orgID, store.Pagination{Limit: 3, Offset: 0})
	require.NoError(t, err)
	require.Len(t, items, 3)
	require.Equal(t, 5, total)

	items2, total2, err := db.ListOrganizationSchedules(ctx, projectID, orgID, store.Pagination{Limit: 3, Offset: 3})
	require.NoError(t, err)
	require.Len(t, items2, 2)
	require.Equal(t, 5, total2)
}

func TestListOrganizationSchedulesHasPendingEvents(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID := createTestOrgForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "org_pending_test", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = db.CreateOrganizationSchedule(ctx, orgID, scheduleID, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	// An unfired event whose fire_at is still in the future is NOT "sending":
	// has_pending_events only reflects events that are actually due. (Regression:
	// recurring schedules pre-generate the next occurrence's event, which kept the
	// "Sending" badge lit indefinitely.)
	items, _, err := db.ListOrganizationSchedules(ctx, projectID, orgID, store.Pagination{Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.False(t, items[0].HasPendingEvents)

	events, err := db.ListPendingOrgScheduledEventsForOrg(ctx, orgID, scheduleID)
	require.NoError(t, err)
	require.Len(t, events, 1)

	// Once the event is due (fire_at <= now), the schedule reports pending events.
	pastTime := time.Now().Add(-1 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = db.ScheduledStore.db.ExecContext(ctx,
		`UPDATE organization_scheduled_events SET fire_at = $1 WHERE id = $2`,
		pastTime, events[0].ID)
	require.NoError(t, err)

	items, _, err = db.ListOrganizationSchedules(ctx, projectID, orgID, store.Pagination{Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.True(t, items[0].HasPendingEvents)

	// Firing the event clears the flag.
	require.NoError(t, db.MarkOrgScheduledEventFired(ctx, events[0].ID))

	items, _, err = db.ListOrganizationSchedules(ctx, projectID, orgID, store.Pagination{Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.False(t, items[0].HasPendingEvents)
}

func TestListOrganizationSchedulesEmpty(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID := createTestOrgForSchedules(t, db, ctx, projectID)

	items, total, err := db.ListOrganizationSchedules(ctx, projectID, orgID, store.Pagination{Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Empty(t, items)
	require.Equal(t, 0, total)
}

func TestListOrganizationSchedulesByScheduleID(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID := createTestOrgForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "org_by_sid", "single")
	require.NoError(t, err)

	ft := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = db.CreateOrganizationSchedule(ctx, orgID, scheduleID, &ft, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	results, err := db.ListOrganizationSchedulesByScheduleID(ctx, orgID, scheduleID)
	require.NoError(t, err)
	require.Len(t, results, 1)
}

func TestListDueOrgScheduledEvents(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID := createTestOrgForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "org_due", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = db.CreateOrganizationSchedule(ctx, orgID, scheduleID, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	events, err := db.ListPendingOrgScheduledEventsForOrg(ctx, orgID, scheduleID)
	require.NoError(t, err)
	require.Len(t, events, 1)

	// Move fire_at to past
	pastTime := time.Now().Add(-1 * time.Hour).UTC()
	_, err = db.ScheduledStore.db.ExecContext(ctx,
		`UPDATE organization_scheduled_events SET fire_at = $1 WHERE id = $2`,
		pastTime, events[0].ID)
	require.NoError(t, err)

	dueEvents, err := db.ListDueOrgScheduledEvents(ctx, 100)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(dueEvents), 1)

	found := false
	for _, e := range dueEvents {
		if e.ID == events[0].ID {
			found = true
			require.Equal(t, "org_due", e.ScheduleName)
			require.Equal(t, projectID, e.ProjectID)
		}
	}
	require.True(t, found, "expected to find the due org event")
}

func TestMarkOrgScheduledEventFired(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID := createTestOrgForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "org_fire", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = db.CreateOrganizationSchedule(ctx, orgID, scheduleID, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	events, err := db.ListPendingOrgScheduledEventsForOrg(ctx, orgID, scheduleID)
	require.NoError(t, err)
	require.Len(t, events, 1)

	err = db.MarkOrgScheduledEventFired(ctx, events[0].ID)
	require.NoError(t, err)

	pending, err := db.ListPendingOrgScheduledEventsForOrg(ctx, orgID, scheduleID)
	require.NoError(t, err)
	require.Empty(t, pending)
}

func TestScanDueOrgScheduledEvents(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID := createTestOrgForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "scan_org_due", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = db.CreateOrganizationSchedule(ctx, orgID, scheduleID, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	events, err := db.ListPendingOrgScheduledEventsForOrg(ctx, orgID, scheduleID)
	require.NoError(t, err)
	require.Len(t, events, 1)

	// Move fire_at to past
	pastTime := time.Now().Add(-1 * time.Hour).UTC()
	_, err = db.ScheduledStore.db.ExecContext(ctx,
		`UPDATE organization_scheduled_events SET fire_at = $1 WHERE id = $2`,
		pastTime, events[0].ID)
	require.NoError(t, err)

	var scanned []DueOrgScheduledEvent
	_, err = db.ScanDueOrgScheduledEvents(ctx, 1000, func(e DueOrgScheduledEvent) error {
		scanned = append(scanned, e)
		return nil
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(scanned), 1)
}

func TestCreateOrganizationScheduleRecurring(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID := createTestOrgForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "org_monthly", "recurring")
	require.NoError(t, err)

	startAt := time.Now().Add(-60 * 24 * time.Hour).UTC().Truncate(time.Microsecond)
	interval := "1 month"

	os, err := db.CreateOrganizationSchedule(ctx, orgID, scheduleID, nil, &startAt, &interval, json.RawMessage(`{}`))
	require.NoError(t, err)

	require.NotNil(t, os.ScheduledAt, "recurring schedule should compute scheduled_at")
	require.True(t, os.ScheduledAt.After(time.Now()), "scheduled_at should be in the future")
	require.Greater(t, os.Occurrence, 0)
	require.NotNil(t, os.Interval)
}

func TestAdvanceAndGenerateOrgScheduleEvents(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID := createTestOrgForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "org_advance", "recurring")
	require.NoError(t, err)

	startAt := time.Now().Add(-30 * 24 * time.Hour).UTC().Truncate(time.Microsecond)
	interval := "7 days"

	os, err := db.CreateOrganizationSchedule(ctx, orgID, scheduleID, nil, &startAt, &interval, json.RawMessage(`{}`))
	require.NoError(t, err)
	require.NotNil(t, os.ScheduledAt)

	originalOccurrence := os.Occurrence

	// Delete all pending events to simulate a completed cycle
	_, err = db.ScheduledStore.db.ExecContext(ctx,
		`DELETE FROM organization_scheduled_events WHERE organization_schedule_id = $1`, os.ID)
	require.NoError(t, err)

	// Move scheduled_at to the past
	_, err = db.ScheduledStore.db.ExecContext(ctx,
		`UPDATE organization_schedules SET scheduled_at = $1 WHERE id = $2`,
		time.Now().Add(-1*time.Hour).UTC(), os.ID)
	require.NoError(t, err)

	os, err = db.GetOrganizationScheduleByID(ctx, os.ID)
	require.NoError(t, err)

	err = db.AdvanceAndGenerateOrgScheduleEvents(ctx, *os)
	require.NoError(t, err)

	advanced, err := db.GetOrganizationScheduleByID(ctx, os.ID)
	require.NoError(t, err)
	require.Greater(t, advanced.Occurrence, originalOccurrence)
	require.True(t, advanced.ScheduledAt.After(time.Now()))

	newEvents, err := db.ListPendingOrgScheduledEventsForOrg(ctx, orgID, scheduleID)
	require.NoError(t, err)
	require.NotEmpty(t, newEvents)
}

func TestScanRecurringOrgSchedulesWithoutPendingEvents(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID := createTestOrgForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "scan_org_recurring", "recurring")
	require.NoError(t, err)

	startAt := time.Now().Add(-30 * 24 * time.Hour).UTC().Truncate(time.Microsecond)
	interval := "7 days"

	os, err := db.CreateOrganizationSchedule(ctx, orgID, scheduleID, nil, &startAt, &interval, json.RawMessage(`{}`))
	require.NoError(t, err)

	// Mark all events fired
	events, err := db.ListPendingOrgScheduledEventsForOrg(ctx, orgID, scheduleID)
	require.NoError(t, err)
	for _, e := range events {
		require.NoError(t, db.MarkOrgScheduledEventFired(ctx, e.ID))
	}

	// Move scheduled_at to the past
	_, err = db.ScheduledStore.db.ExecContext(ctx,
		`UPDATE organization_schedules SET scheduled_at = $1 WHERE id = $2`,
		time.Now().Add(-1*time.Hour).UTC(), os.ID)
	require.NoError(t, err)

	var found []OrganizationSchedule
	_, err = db.ScanRecurringOrgSchedulesWithoutPendingEvents(ctx, 1000, func(os OrganizationSchedule) error {
		found = append(found, os)
		return nil
	})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(found), 1)

	foundOurs := false
	for _, f := range found {
		if f.ID == os.ID {
			foundOurs = true
		}
	}
	require.True(t, foundOurs)
}

func TestBackfillOrgScheduledEventsForOffset(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID := createTestOrgForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "backfill_org", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(7 * 24 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = db.CreateOrganizationSchedule(ctx, orgID, scheduleID, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	events, err := db.ListPendingOrgScheduledEventsForOrg(ctx, orgID, scheduleID)
	require.NoError(t, err)
	require.Len(t, events, 1)

	newOffset, err := db.CreateScheduleOffset(ctx, scheduleID, "3 hours", "before")
	require.NoError(t, err)

	count, err := db.BackfillOrgScheduledEventsForOffset(ctx, scheduleID, newOffset.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	events, err = db.ListPendingOrgScheduledEventsForOrg(ctx, orgID, scheduleID)
	require.NoError(t, err)
	require.Len(t, events, 2)
}

func TestMultipleUsersOnSameSchedule(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	scheduleID, err := db.UpsertSchedule(ctx, projectID, "team_meeting", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Microsecond)

	user1 := createTestUserForSchedules(t, db, ctx, projectID)
	user2 := createTestUserForSchedules(t, db, ctx, projectID)
	user3 := createTestUserForSchedules(t, db, ctx, projectID)

	for _, uid := range []uuid.UUID{user1, user2, user3} {
		_, err = db.CreateUserSchedule(ctx, uid, scheduleID, &futureTime, nil, nil, json.RawMessage(`{}`))
		require.NoError(t, err)
	}

	// Each user should have their own pending event
	for _, uid := range []uuid.UUID{user1, user2, user3} {
		events, err := db.ListPendingScheduledEventsForUser(ctx, uid, scheduleID)
		require.NoError(t, err)
		require.Len(t, events, 1)
	}

	// Backfill with a new offset should create events for all users
	newOffset, err := db.CreateScheduleOffset(ctx, scheduleID, "15 minutes", "before")
	require.NoError(t, err)

	count, err := db.BackfillUserScheduledEventsForOffset(ctx, scheduleID, newOffset.ID)
	require.NoError(t, err)
	require.Equal(t, int64(3), count)
}

func TestMultipleOrgsOnSameSchedule(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	scheduleID, err := db.UpsertSchedule(ctx, projectID, "org_meeting", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Microsecond)

	org1 := createTestOrgForSchedules(t, db, ctx, projectID)
	org2 := createTestOrgForSchedules(t, db, ctx, projectID)

	for _, oid := range []uuid.UUID{org1, org2} {
		_, err = db.CreateOrganizationSchedule(ctx, oid, scheduleID, &futureTime, nil, nil, json.RawMessage(`{}`))
		require.NoError(t, err)
	}

	for _, oid := range []uuid.UUID{org1, org2} {
		events, err := db.ListPendingOrgScheduledEventsForOrg(ctx, oid, scheduleID)
		require.NoError(t, err)
		require.Len(t, events, 1)
	}

	newOffset, err := db.CreateScheduleOffset(ctx, scheduleID, "10 minutes", "after")
	require.NoError(t, err)

	count, err := db.BackfillOrgScheduledEventsForOffset(ctx, scheduleID, newOffset.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
}

func TestScheduleSchemaRowsToSchedules(t *testing.T) {
	t.Parallel()

	id1 := uuid.New()
	id2 := uuid.New()
	pid := uuid.New()

	rows := scheduleSchemaRows{
		{ID: id1, ProjectID: pid, Name: "a", Type: "single", Path: ptr.To(".foo"), Types: []string{"string"}},
		{ID: id1, ProjectID: pid, Name: "a", Type: "single", Path: ptr.To(".bar"), Types: []string{"number"}},
		{ID: id2, ProjectID: pid, Name: "b", Type: "recurring", Path: nil, Types: nil},
	}

	schedules := rows.ToSchedules()
	require.Len(t, schedules, 2)

	// First schedule should have 2 schema paths
	require.Equal(t, id1, schedules[0].ID)
	require.Equal(t, "a", schedules[0].Name)
	require.Len(t, schedules[0].Schema, 2)

	// Second schedule should have 0 schema paths (path was nil)
	require.Equal(t, id2, schedules[1].ID)
	require.Equal(t, "b", schedules[1].Name)
	require.Len(t, schedules[1].Schema, 0)
}

func TestCreateUserScheduleNoEventsForPastSingleSchedule(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "past_single", "single")
	require.NoError(t, err)

	// The scheduled time is in the past. generateScheduledEvents only creates
	// events where fire_at > NOW(), so we expect no events.
	pastTime := time.Now().Add(-24 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = db.CreateUserSchedule(ctx, userID, scheduleID, &pastTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	events, err := db.ListPendingScheduledEventsForUser(ctx, userID, scheduleID)
	require.NoError(t, err)
	require.Empty(t, events, "should not generate events for past fire_at")
}

func TestCreateOrgScheduleNoEventsForPastSingleSchedule(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID := createTestOrgForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "org_past_single", "single")
	require.NoError(t, err)

	pastTime := time.Now().Add(-24 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = db.CreateOrganizationSchedule(ctx, orgID, scheduleID, &pastTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	events, err := db.ListPendingOrgScheduledEventsForOrg(ctx, orgID, scheduleID)
	require.NoError(t, err)
	require.Empty(t, events, "should not generate events for past fire_at")
}

func TestAdvanceUserScheduleNilIntervalIsNoop(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "noop_advance", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	us, err := db.CreateUserSchedule(ctx, userID, scheduleID, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	// Advance on a non-recurring schedule should be a no-op
	err = db.AdvanceAndGenerateUserScheduleEvents(ctx, *us)
	require.NoError(t, err)
}

func TestAdvanceOrgScheduleNilIntervalIsNoop(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID := createTestOrgForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "org_noop_advance", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	os, err := db.CreateOrganizationSchedule(ctx, orgID, scheduleID, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	err = db.AdvanceAndGenerateOrgScheduleEvents(ctx, *os)
	require.NoError(t, err)
}

func TestCreateScheduleOffsetWithBeforeDirection(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	scheduleID, err := db.UpsertSchedule(ctx, projectID, "before_dir", "single")
	require.NoError(t, err)

	// Create a "before" offset
	offset, err := db.CreateScheduleOffset(ctx, scheduleID, "1 day", "before")
	require.NoError(t, err)
	require.Equal(t, "before", offset.Direction)

	// Create a user schedule far enough in the future
	userID := createTestUserForSchedules(t, db, ctx, projectID)
	futureTime := time.Now().Add(7 * 24 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = db.CreateUserSchedule(ctx, userID, scheduleID, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	events, err := db.ListPendingScheduledEventsForUser(ctx, userID, scheduleID)
	require.NoError(t, err)
	// Default (after 0min) + 1 day before = 2 events
	require.Len(t, events, 2)

	// Verify the "before" event fires before the anchor
	for _, e := range events {
		if e.ScheduleOffsetID == offset.ID {
			require.True(t, e.FireAt.Before(futureTime), "before-offset event should fire before the anchor")
		}
	}
}

func TestUpsertUserScheduleRecurring(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "upsert_recurring", "recurring")
	require.NoError(t, err)

	// Deliberately not a whole multiple of the interval. The store returns the
	// first occurrence strictly after the database's clock, so an anchor exactly
	// N intervals back puts that occurrence on this very instant -- and a
	// database a millisecond behind this process returns it rather than the one
	// after, already past by the time it is asserted on.
	startAt := time.Now().Add(-10 * 24 * time.Hour).UTC().Truncate(time.Microsecond)
	interval := "7 days"

	us, err := db.UpsertUserSchedule(ctx, uuid.Nil, userID, scheduleID, nil, &startAt, &interval, json.RawMessage(`{}`))
	require.NoError(t, err)
	require.NotNil(t, us.ScheduledAt)
	require.True(t, us.ScheduledAt.After(time.Now()))
	require.Greater(t, us.Occurrence, 0)

	// Upsert again with the same id and different data – same row
	us2, err := db.UpsertUserSchedule(ctx, us.ID, userID, scheduleID, nil, &startAt, &interval, json.RawMessage(`{"updated":true}`))
	require.NoError(t, err)
	require.Equal(t, us.ID, us2.ID)
}

func TestUpsertOrganizationScheduleRecurring(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID := createTestOrgForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "org_upsert_recurring", "recurring")
	require.NoError(t, err)

	// Deliberately not a whole multiple of the interval. The store returns the
	// first occurrence strictly after the database's clock, so an anchor exactly
	// N intervals back puts that occurrence on this very instant -- and a
	// database a millisecond behind this process returns it rather than the one
	// after, already past by the time it is asserted on.
	startAt := time.Now().Add(-10 * 24 * time.Hour).UTC().Truncate(time.Microsecond)
	interval := "7 days"

	os, err := db.UpsertOrganizationSchedule(ctx, uuid.Nil, orgID, scheduleID, nil, &startAt, &interval, json.RawMessage(`{}`))
	require.NoError(t, err)
	require.NotNil(t, os.ScheduledAt)
	require.True(t, os.ScheduledAt.After(time.Now()))
	require.Greater(t, os.Occurrence, 0)

	os2, err := db.UpsertOrganizationSchedule(ctx, os.ID, orgID, scheduleID, nil, &startAt, &interval, json.RawMessage(`{"updated":true}`))
	require.NoError(t, err)
	require.Equal(t, os.ID, os2.ID)
}

func TestScheduleIsolationBetweenProjects(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	ctx := context.Background()

	project1 := uuid.New()
	project2 := uuid.New()

	id1, err := db.UpsertSchedule(ctx, project1, "same_name", "single")
	require.NoError(t, err)

	id2, err := db.UpsertSchedule(ctx, project2, "same_name", "recurring")
	require.NoError(t, err)

	require.NotEqual(t, id1, id2, "same name in different projects should produce different IDs")

	sched1, err := db.GetScheduleByName(ctx, project1, "same_name")
	require.NoError(t, err)
	require.Equal(t, "single", sched1.Type)

	sched2, err := db.GetScheduleByName(ctx, project2, "same_name")
	require.NoError(t, err)
	require.Equal(t, "recurring", sched2.Type)

	list1, err := db.ListSchedules(ctx, project1)
	require.NoError(t, err)
	require.Len(t, list1, 1)

	list2, err := db.ListSchedules(ctx, project2)
	require.NoError(t, err)
	require.Len(t, list2, 1)
}

func TestUserScheduleDataPropagatedToEvents(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID := createTestUserForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "data_prop", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	data := json.RawMessage(`{"appointment":"dentist","time":"10:00"}`)
	_, err = db.CreateUserSchedule(ctx, userID, scheduleID, &futureTime, nil, nil, data)
	require.NoError(t, err)

	events, err := db.ListPendingScheduledEventsForUser(ctx, userID, scheduleID)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.JSONEq(t, `{"appointment":"dentist","time":"10:00"}`, string(events[0].Data))
}

func TestOrgScheduleDataPropagatedToEvents(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID := createTestOrgForSchedules(t, db, ctx, projectID)
	scheduleID, err := db.UpsertSchedule(ctx, projectID, "org_data_prop", "single")
	require.NoError(t, err)

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	data := json.RawMessage(`{"meeting":"standup"}`)
	_, err = db.CreateOrganizationSchedule(ctx, orgID, scheduleID, &futureTime, nil, nil, data)
	require.NoError(t, err)

	events, err := db.ListPendingOrgScheduledEventsForOrg(ctx, orgID, scheduleID)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.JSONEq(t, `{"meeting":"standup"}`, string(events[0].Data))
}

// nextOccurrence calls the next_schedule_occurrence SQL function with an explicit
// as_of so results are deterministic, and returns the occurrence number, its
// timestamp, and whether the defining properties (1)-(3) hold for that result.
type occurrenceProps struct {
	Occurrence int64     `db:"occurrence"`
	NextAt     time.Time `db:"next_at"`
	P1Future   bool      `db:"p1_future"`
	P2First    bool      `db:"p2_first"`
	P3Forward  bool      `db:"p3_forward"`
}

func nextOccurrence(t *testing.T, db *State, anchor time.Time, interval string, cur int64, asOf time.Time) occurrenceProps {
	t.Helper()
	// Properties are evaluated with interval arithmetic in Postgres (the only place
	// calendar math is exact). next_at > as_of (in the future); the previous
	// occurrence is at or before as_of unless it is the first one (it is the
	// earliest such occurrence); and the occurrence never moves backwards.
	const q = `
	WITH r AS (
		SELECT next_at, occurrence
		FROM next_schedule_occurrence($1::timestamptz, $2::interval, $3::bigint, $4::timestamptz)
	)
	SELECT
		r.occurrence,
		r.next_at,
		(r.next_at > $4)                                                              AS p1_future,
		(r.occurrence = $3 + 1 OR $1::timestamptz + (r.occurrence - 1) * $2::interval <= $4) AS p2_first,
		(r.occurrence >= $3 + 1)                                                      AS p3_forward
	FROM r`

	var pr occurrenceProps
	err := db.ScheduledStore.db.QueryRowxContext(context.Background(), q, anchor, interval, cur, asOf).StructScan(&pr)
	require.NoError(t, err)
	return pr
}

func TestNextScheduleOccurrenceExactCases(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)

	ts := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		require.NoError(t, err)
		return v.UTC()
	}

	type testCase struct {
		anchor   string
		interval string
		cur      int64
		asOf     string
		wantOcc  int64
		wantNext string
	}

	cases := map[string]testCase{
		// The stuck-"Sending" anniversary: yearly, first send is anchor + 1 year.
		"yearly anniversary": {"2026-03-28T15:45:52Z", "1 year", 0, "2026-06-25T00:00:00Z", 1, "2027-03-28T15:45:52Z"},
		// Leap-day anchor + yearly clamps to Feb 28 in common years (occurrence 3 = 2027).
		"leap-day yearly clamps to Feb 28": {"2024-02-29T12:00:00Z", "1 year", 0, "2027-01-01T00:00:00Z", 3, "2027-02-28T12:00:00Z"},
		// Leap-day anchor + yearly lands back on Feb 29 every 4th year (occurrence 4 = 2028).
		"leap-day yearly lands on Feb 29": {"2024-02-29T12:00:00Z", "1 year", 0, "2027-06-01T00:00:00Z", 4, "2028-02-29T12:00:00Z"},
		// as_of exactly on an occurrence boundary must pick the next one (strict >).
		"daily on boundary picks next": {"2026-01-01T00:00:00Z", "1 day", 0, "2026-01-10T00:00:00Z", 10, "2026-01-11T00:00:00Z"},
		// Month-end clamping: Jan 31 + 1 month = Feb 29 (leap), + 2 months = Mar 31.
		"month-end clamp from Jan 31": {"2024-01-31T12:00:00Z", "1 month", 0, "2024-03-15T00:00:00Z", 2, "2024-03-31T12:00:00Z"},
		// Advancing from a non-zero current occurrence only moves forward.
		"advance from current occurrence": {"2026-01-01T00:00:00Z", "1 day", 5, "2026-01-03T00:00:00Z", 6, "2026-01-07T00:00:00Z"},
		// ~470 years of dormancy on a 1-minute interval: far past the old 10000 cap.
		"far dormancy beyond old cap": {"2026-01-01T00:00:00Z", "1 minute", 0, "2496-01-01T00:00:00Z", 247196161, "2496-01-01T00:01:00Z"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pr := nextOccurrence(t, db, ts(tc.anchor), tc.interval, tc.cur, ts(tc.asOf))
			require.Equal(t, tc.wantOcc, pr.Occurrence)
			require.Truef(t, pr.NextAt.Equal(ts(tc.wantNext)), "next_at = %s, want %s", pr.NextAt, tc.wantNext)
			require.True(t, pr.P1Future && pr.P2First && pr.P3Forward, "defining properties must hold")
		})
	}
}

// TestNextScheduleOccurrenceProperties asserts the result is exact across a broad
// grid (leap years, month-end clamping, mixed calendar intervals, horizons out to
// year 3025) by checking the three defining properties for every combination.
func TestNextScheduleOccurrenceProperties(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)

	ts := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		require.NoError(t, err)
		return v.UTC()
	}

	anchors := []string{
		"2024-01-31T12:00:00Z", "2020-12-31T06:00:00Z",
		"2026-03-28T15:45:52Z", "1999-11-15T09:00:00Z",
	}
	intervals := []string{
		"1 minute", "1 hour", "1 day", "1 week", "30 days",
		"1 month", "3 months", "1 year", "5 years", "1 mon 15 days",
	}
	curs := []int64{0, 1, 7, 99}
	asOfs := []string{
		"2026-06-25T14:00:00Z", "2030-02-28T00:00:00Z",
		"2520-06-01T00:00:00Z", "3025-09-09T00:00:00Z",
	}

	for _, a := range anchors {
		for _, iv := range intervals {
			for _, cur := range curs {
				for _, asOf := range asOfs {
					pr := nextOccurrence(t, db, ts(a), iv, cur, ts(asOf))
					require.Truef(t, pr.P1Future && pr.P2First && pr.P3Forward,
						"anchor=%s interval=%s cur=%d asOf=%s -> occ=%d (p1=%v p2=%v p3=%v)",
						a, iv, cur, asOf, pr.Occurrence, pr.P1Future, pr.P2First, pr.P3Forward)
				}
			}
		}
	}
}

func TestNextScheduleOccurrenceRejectsNonPositiveInterval(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)

	const q = `SELECT next_at FROM next_schedule_occurrence($1::timestamptz, $2::interval, 0, $3::timestamptz)`
	for _, interval := range []string{"0 seconds", "-1 day"} {
		var nextAt time.Time
		err := db.ScheduledStore.db.QueryRowxContext(context.Background(), q,
			time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), interval,
			time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)).Scan(&nextAt)
		require.Error(t, err, "interval %q should be rejected", interval)
		require.Contains(t, err.Error(), "interval must be positive")
	}
}
