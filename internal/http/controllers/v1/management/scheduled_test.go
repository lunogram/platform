package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type testScheduledController struct {
	controller *ScheduledController
	projectID  uuid.UUID
	store      *subjects.State
	actorCtx   context.Context
}

func setupScheduledController(t *testing.T) *testScheduledController {
	t.Helper()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmtDB, usrsDB, _ := teststore.RunPostgreSQL(t)

	pub := pubsub.NewNoopPublisher()

	orgsStore := management.NewOrganizationsStore(mgmtDB)
	orgID, err := orgsStore.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projectsStore := management.NewProjectsStore(mgmtDB)
	projectID, err := projectsStore.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           DefaultProject.Name,
		Timezone:       DefaultProject.Timezone,
		Locale:         DefaultProject.Locale,
	})
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(orgID),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	usersState := subjects.NewState(usrsDB, logger)
	controller := NewScheduledController(logger, usersState, pub, engine)

	return &testScheduledController{
		controller: controller,
		projectID:  projectID,
		store:      usersState,
		actorCtx:   actorCtx,
	}
}

func (tc *testScheduledController) createUser(t *testing.T) uuid.UUID {
	t.Helper()
	anonID := uuid.New().String()
	userID, err := tc.store.CreateUser(context.Background(), subjects.User{
		ProjectID:   tc.projectID,
		AnonymousID: &anonID,
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	return userID
}

func (tc *testScheduledController) createOrg(t *testing.T) uuid.UUID {
	t.Helper()
	orgID, err := tc.store.UpsertOrganization(context.Background(), tc.projectID, subjects.UpsertOrganizationParams{
		ExternalID: uuid.New().String(),
		Name:       ptr("Test Org"),
	})
	require.NoError(t, err)
	return orgID
}

func (tc *testScheduledController) createSchedule(t *testing.T, name, scheduleType string) uuid.UUID {
	t.Helper()
	id, err := tc.store.UpsertSchedule(context.Background(), tc.projectID, name, scheduleType)
	require.NoError(t, err)
	return id
}

func TestListScheduledSchemas(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	ctx := context.Background()

	sid1 := tc.createSchedule(t, "birthday", "single")
	sid2 := tc.createSchedule(t, "renewal", "recurring")

	err := tc.store.UpsertScheduledSchema(ctx, tc.projectID, sid1, rules.Paths{
		{Path: ".amount", Type: rules.TypeNumber},
		{Path: ".currency", Type: rules.TypeString},
	})
	require.NoError(t, err)

	_ = sid2 // no schema for renewal

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/subjects/user/scheduled/schema", nil)
	req = req.WithContext(tc.actorCtx)

	tc.controller.ListScheduledSchemas(res, req, tc.projectID)

	require.Equal(t, 200, res.Code)

	var response oapi.ScheduledEventListResponse
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Len(t, response.Results, 2)

	byName := make(map[string]oapi.ScheduledEventWithSchema)
	for _, r := range response.Results {
		byName[r.Name] = r
	}

	bday, ok := byName["birthday"]
	require.True(t, ok)
	require.Equal(t, sid1, bday.Id)
	require.Len(t, bday.Schema, 2)

	pathMap := make(map[string][]string)
	for _, s := range bday.Schema {
		pathMap[s.Path] = s.Types
	}
	require.Contains(t, pathMap, ".data.amount")
	require.Contains(t, pathMap, ".data.currency")

	renewal, ok := byName["renewal"]
	require.True(t, ok)
	require.Equal(t, sid2, renewal.Id)
	require.Empty(t, renewal.Schema)

	// Each schedule should have at least the default offset
	require.GreaterOrEqual(t, len(bday.Offsets), 1)
	require.GreaterOrEqual(t, len(renewal.Offsets), 1)
}

func TestListScheduledSchemasEmpty(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/subjects/user/scheduled/schema", nil)
	req = req.WithContext(tc.actorCtx)

	tc.controller.ListScheduledSchemas(res, req, tc.projectID)

	require.Equal(t, 200, res.Code)

	var response oapi.ScheduledEventListResponse
	err := json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Empty(t, response.Results)
}

func TestListScheduledSchemasUnauthorized(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+tc.projectID.String()+"/subjects/user/scheduled/schema", nil)
	// No actor context

	tc.controller.ListScheduledSchemas(res, req, tc.projectID)

	require.Equal(t, 401, res.Code)
}

func TestDeleteScheduledSchema(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)

	sid := tc.createSchedule(t, "to_delete", "single")

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/admin/projects/"+tc.projectID.String()+"/subjects/user/scheduled/schema/"+sid.String(), nil)
	req = req.WithContext(tc.actorCtx)

	tc.controller.DeleteScheduledSchema(res, req, tc.projectID, sid)

	require.Equal(t, 200, res.Code)

	// Verify it's gone
	_, err := tc.store.GetScheduleByID(context.Background(), sid)
	require.Error(t, err)
}

func TestDeleteScheduledSchemaNotFound(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/admin/projects/"+tc.projectID.String()+"/subjects/user/scheduled/schema/"+uuid.New().String(), nil)
	req = req.WithContext(tc.actorCtx)

	tc.controller.DeleteScheduledSchema(res, req, tc.projectID, uuid.New())

	require.Equal(t, 404, res.Code)
}

func TestDeleteScheduledSchemaUnauthorized(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	sid := tc.createSchedule(t, "unauth_del", "single")

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/admin/projects/"+tc.projectID.String()+"/subjects/user/scheduled/schema/"+sid.String(), nil)

	tc.controller.DeleteScheduledSchema(res, req, tc.projectID, sid)

	require.Equal(t, 401, res.Code)
}

func TestCreateScheduleOffset(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)

	sid := tc.createSchedule(t, "offset_test", "single")

	body, _ := json.Marshal(oapi.CreateScheduleOffsetRequest{
		Offset:    "30 minutes",
		Direction: "before",
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/admin/projects/"+tc.projectID.String()+"/subjects/user/scheduled/schema/"+sid.String()+"/offsets", bytes.NewReader(body))
	req = req.WithContext(tc.actorCtx)

	tc.controller.CreateScheduleOffset(res, req, tc.projectID, sid)

	require.Equal(t, 201, res.Code)

	var offset oapi.ScheduleOffset
	err := json.Unmarshal(res.Body.Bytes(), &offset)
	require.NoError(t, err)
	require.Equal(t, sid, offset.ScheduleId)
	require.Equal(t, oapi.ScheduleOffsetDirection("before"), offset.Direction)
	require.NotEqual(t, uuid.Nil, offset.Id)
}

func TestCreateScheduleOffsetScheduleNotFound(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)

	body, _ := json.Marshal(oapi.CreateScheduleOffsetRequest{
		Offset:    "1 hour",
		Direction: "after",
	})

	fakeID := uuid.New()
	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	req = req.WithContext(tc.actorCtx)

	tc.controller.CreateScheduleOffset(res, req, tc.projectID, fakeID)

	require.Equal(t, 404, res.Code)
}

func TestCreateScheduleOffsetWrongProject(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	sid := tc.createSchedule(t, "wrong_project", "single")

	body, _ := json.Marshal(oapi.CreateScheduleOffsetRequest{
		Offset:    "1 hour",
		Direction: "after",
	})

	// The actor only has access to tc.projectID, so passing a different
	// project ID is rejected by RBAC before the schedule ownership check.
	otherProjectID := uuid.New()
	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	req = req.WithContext(tc.actorCtx)

	tc.controller.CreateScheduleOffset(res, req, otherProjectID, sid)

	require.Equal(t, 403, res.Code)
}

func TestCreateScheduleOffsetUnauthorized(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	sid := tc.createSchedule(t, "unauth_offset", "single")

	body, _ := json.Marshal(oapi.CreateScheduleOffsetRequest{
		Offset:    "1 hour",
		Direction: "after",
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))

	tc.controller.CreateScheduleOffset(res, req, tc.projectID, sid)

	require.Equal(t, 401, res.Code)
}

func TestCreateScheduleOffsetDuplicate(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	sid := tc.createSchedule(t, "dup_offset", "single")

	body, _ := json.Marshal(oapi.CreateScheduleOffsetRequest{
		Offset:    "45 minutes",
		Direction: "before",
	})

	res1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	req1 = req1.WithContext(tc.actorCtx)
	tc.controller.CreateScheduleOffset(res1, req1, tc.projectID, sid)
	require.Equal(t, 201, res1.Code)

	body2, _ := json.Marshal(oapi.CreateScheduleOffsetRequest{
		Offset:    "45 minutes",
		Direction: "before",
	})

	res2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/", bytes.NewReader(body2))
	req2 = req2.WithContext(tc.actorCtx)
	tc.controller.CreateScheduleOffset(res2, req2, tc.projectID, sid)
	require.Equal(t, 409, res2.Code)
}

func TestGetUserScheduled(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	ctx := context.Background()

	userID := tc.createUser(t)
	sid := tc.createSchedule(t, "user_list", "single")

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err := tc.store.CreateUserSchedule(ctx, userID, sid, &futureTime, nil, nil, json.RawMessage(`{"k":"v"}`))
	require.NoError(t, err)

	limit := oapi.PaginationLimit(20)
	offset := oapi.PaginationOffset(0)
	params := oapi.GetUserScheduledParams{
		Limit:  &limit,
		Offset: &offset,
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(tc.actorCtx)

	tc.controller.GetUserScheduled(res, req, tc.projectID, userID, params)

	require.Equal(t, 200, res.Code)

	var response oapi.UserScheduledList
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, 1, response.Total)
	require.Len(t, response.Results, 1)
	require.Equal(t, userID, response.Results[0].UserId)
	require.Equal(t, sid, response.Results[0].ScheduledId)
}

func TestGetUserScheduledEmpty(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	userID := tc.createUser(t)

	limit := oapi.PaginationLimit(20)
	offset := oapi.PaginationOffset(0)
	params := oapi.GetUserScheduledParams{
		Limit:  &limit,
		Offset: &offset,
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(tc.actorCtx)

	tc.controller.GetUserScheduled(res, req, tc.projectID, userID, params)

	require.Equal(t, 200, res.Code)

	var response oapi.UserScheduledList
	err := json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, 0, response.Total)
	require.Empty(t, response.Results)
}

func TestGetUserScheduledUserNotFound(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)

	limit := oapi.PaginationLimit(20)
	offset := oapi.PaginationOffset(0)
	params := oapi.GetUserScheduledParams{
		Limit:  &limit,
		Offset: &offset,
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(tc.actorCtx)

	tc.controller.GetUserScheduled(res, req, tc.projectID, uuid.New(), params)

	require.Equal(t, 404, res.Code)
}

func TestGetUserScheduledUnauthorized(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	userID := tc.createUser(t)

	limit := oapi.PaginationLimit(20)
	offset := oapi.PaginationOffset(0)
	params := oapi.GetUserScheduledParams{
		Limit:  &limit,
		Offset: &offset,
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	tc.controller.GetUserScheduled(res, req, tc.projectID, userID, params)

	require.Equal(t, 401, res.Code)
}

func TestUpsertUserScheduled(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)

	userID := tc.createUser(t)
	sid := tc.createSchedule(t, "user_upsert", "single")

	futureTime := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Microsecond)
	data := json.RawMessage(`{"reminder":"dentist"}`)
	body, _ := json.Marshal(oapi.UpsertUserScheduledRequest{
		ScheduledId: &sid,
		ScheduledAt: &futureTime,
		Data:        &data,
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/", bytes.NewReader(body))
	req = req.WithContext(tc.actorCtx)

	tc.controller.UpsertUserScheduled(res, req, tc.projectID, userID)

	require.Equal(t, 200, res.Code)

	var result oapi.UserScheduled
	err := json.Unmarshal(res.Body.Bytes(), &result)
	require.NoError(t, err)
	require.Equal(t, userID, result.UserId)
	require.Equal(t, sid, result.ScheduledId)
	require.WithinDuration(t, futureTime, result.ScheduledAt, time.Second)
}

func TestUpsertUserScheduledRecurring(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)

	userID := tc.createUser(t)
	sid := tc.createSchedule(t, "user_recurring", "recurring")

	startAt := time.Now().Add(-14 * 24 * time.Hour).UTC().Truncate(time.Microsecond)
	interval := "7 days"
	data := json.RawMessage(`{}`)
	body, _ := json.Marshal(oapi.UpsertUserScheduledRequest{
		ScheduledId: &sid,
		StartAt:     &startAt,
		Interval:    &interval,
		Data:        &data,
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/", bytes.NewReader(body))
	req = req.WithContext(tc.actorCtx)

	tc.controller.UpsertUserScheduled(res, req, tc.projectID, userID)

	require.Equal(t, 200, res.Code)

	var result oapi.UserScheduled
	err := json.Unmarshal(res.Body.Bytes(), &result)
	require.NoError(t, err)
	require.Equal(t, userID, result.UserId)
	require.NotNil(t, result.Interval)
	require.True(t, result.ScheduledAt.After(time.Now()), "recurring scheduled_at should be in the future")
}

func TestUpsertUserScheduledRecurringWithoutStartAt(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)

	userID := tc.createUser(t)
	sid := tc.createSchedule(t, "user_recurring_no_start", "recurring")

	interval := "7 days"
	data := json.RawMessage(`{}`)
	body, _ := json.Marshal(oapi.UpsertUserScheduledRequest{
		ScheduledId: &sid,
		Interval:    &interval,
		Data:        &data,
		// No StartAt, no ScheduledAt — start_at should default to now
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/", bytes.NewReader(body))
	req = req.WithContext(tc.actorCtx)

	tc.controller.UpsertUserScheduled(res, req, tc.projectID, userID)

	require.Equal(t, 200, res.Code)

	var result oapi.UserScheduled
	err := json.Unmarshal(res.Body.Bytes(), &result)
	require.NoError(t, err)
	require.Equal(t, userID, result.UserId)
	require.NotNil(t, result.Interval)
	require.NotNil(t, result.StartAt, "start_at should be defaulted when not provided")
	require.WithinDuration(t, time.Now(), *result.StartAt, 5*time.Second, "start_at should default to approximately now")
	require.False(t, result.ScheduledAt.IsZero(), "scheduled_at should not be zero time")
	require.True(t, result.ScheduledAt.After(time.Now()), "recurring scheduled_at should be in the future")
}

func TestUpsertUserScheduledMissingScheduledAt(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)

	userID := tc.createUser(t)
	sid := tc.createSchedule(t, "missing_at", "single")

	data := json.RawMessage(`{}`)
	body, _ := json.Marshal(oapi.UpsertUserScheduledRequest{
		ScheduledId: &sid,
		Data:        &data,
		// No ScheduledAt, no Interval => bad request
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/", bytes.NewReader(body))
	req = req.WithContext(tc.actorCtx)

	tc.controller.UpsertUserScheduled(res, req, tc.projectID, userID)

	require.Equal(t, 400, res.Code)
}

func TestUpsertUserScheduledUserNotFound(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	sid := tc.createSchedule(t, "user_nf", "single")

	futureTime := time.Now().Add(24 * time.Hour).UTC()
	data := json.RawMessage(`{}`)
	body, _ := json.Marshal(oapi.UpsertUserScheduledRequest{
		ScheduledId: &sid,
		ScheduledAt: &futureTime,
		Data:        &data,
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/", bytes.NewReader(body))
	req = req.WithContext(tc.actorCtx)

	tc.controller.UpsertUserScheduled(res, req, tc.projectID, uuid.New())

	require.Equal(t, 404, res.Code)
}

func TestUpsertUserScheduledUnauthorized(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	userID := tc.createUser(t)
	sid := tc.createSchedule(t, "unauth_upsert", "single")

	futureTime := time.Now().Add(24 * time.Hour).UTC()
	data := json.RawMessage(`{}`)
	body, _ := json.Marshal(oapi.UpsertUserScheduledRequest{
		ScheduledId: &sid,
		ScheduledAt: &futureTime,
		Data:        &data,
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/", bytes.NewReader(body))

	tc.controller.UpsertUserScheduled(res, req, tc.projectID, userID)

	require.Equal(t, 401, res.Code)
}

func TestUpsertUserScheduledIdempotent(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)

	userID := tc.createUser(t)
	sid := tc.createSchedule(t, "user_idem", "single")

	time1 := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	data := json.RawMessage(`{}`)
	body1, _ := json.Marshal(oapi.UpsertUserScheduledRequest{
		ScheduledId: &sid,
		ScheduledAt: &time1,
		Data:        &data,
	})

	res1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("PUT", "/", bytes.NewReader(body1))
	req1 = req1.WithContext(tc.actorCtx)
	tc.controller.UpsertUserScheduled(res1, req1, tc.projectID, userID)
	require.Equal(t, 200, res1.Code)

	var result1 oapi.UserScheduled
	require.NoError(t, json.Unmarshal(res1.Body.Bytes(), &result1))

	time2 := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Microsecond)
	body2, _ := json.Marshal(oapi.UpsertUserScheduledRequest{
		ScheduledId: &sid,
		ScheduledAt: &time2,
		Data:        &data,
	})

	res2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("PUT", "/", bytes.NewReader(body2))
	req2 = req2.WithContext(tc.actorCtx)
	tc.controller.UpsertUserScheduled(res2, req2, tc.projectID, userID)
	require.Equal(t, 200, res2.Code)

	var result2 oapi.UserScheduled
	require.NoError(t, json.Unmarshal(res2.Body.Bytes(), &result2))

	require.Equal(t, result1.Id, result2.Id, "upsert should return same instance ID")
	require.WithinDuration(t, time2, result2.ScheduledAt, time.Second)
}

func TestDeleteUserScheduled(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	ctx := context.Background()

	userID := tc.createUser(t)
	sid := tc.createSchedule(t, "user_del", "single")

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	us, err := tc.store.CreateUserSchedule(ctx, userID, sid, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/", nil)
	req = req.WithContext(tc.actorCtx)

	tc.controller.DeleteUserScheduled(res, req, tc.projectID, userID, us.ID)

	require.Equal(t, 204, res.Code)

	_, err = tc.store.GetUserScheduleByID(ctx, us.ID)
	require.Error(t, err)
}

func TestDeleteUserScheduledNotFound(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	userID := tc.createUser(t)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/", nil)
	req = req.WithContext(tc.actorCtx)

	tc.controller.DeleteUserScheduled(res, req, tc.projectID, userID, uuid.New())

	require.Equal(t, 404, res.Code)
}

func TestDeleteUserScheduledWrongUser(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	ctx := context.Background()

	user1 := tc.createUser(t)
	user2 := tc.createUser(t)
	sid := tc.createSchedule(t, "wrong_user_del", "single")

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	us, err := tc.store.CreateUserSchedule(ctx, user1, sid, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/", nil)
	req = req.WithContext(tc.actorCtx)

	tc.controller.DeleteUserScheduled(res, req, tc.projectID, user2, us.ID)

	require.Equal(t, 404, res.Code)
}

func TestDeleteUserScheduledUnauthorized(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	userID := tc.createUser(t)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/", nil)

	tc.controller.DeleteUserScheduled(res, req, tc.projectID, userID, uuid.New())

	require.Equal(t, 401, res.Code)
}

func TestUpdateUserScheduled(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	ctx := context.Background()

	userID := tc.createUser(t)
	sid := tc.createSchedule(t, "user_update", "single")

	original := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	us, err := tc.store.CreateUserSchedule(ctx, userID, sid, &original, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	newTime := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Microsecond)
	body, _ := json.Marshal(oapi.UpdateUserScheduledRequest{
		ScheduledAt: &newTime,
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/", bytes.NewReader(body))
	req = req.WithContext(tc.actorCtx)

	tc.controller.UpdateUserScheduled(res, req, tc.projectID, userID, us.ID)

	require.Equal(t, 200, res.Code)

	var result oapi.UserScheduled
	err = json.Unmarshal(res.Body.Bytes(), &result)
	require.NoError(t, err)
	require.WithinDuration(t, newTime, result.ScheduledAt, time.Second)
}

func TestUpdateUserScheduledUnauthorized(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)

	newTime := time.Now().Add(72 * time.Hour).UTC()
	body, _ := json.Marshal(oapi.UpdateUserScheduledRequest{
		ScheduledAt: &newTime,
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/", bytes.NewReader(body))

	tc.controller.UpdateUserScheduled(res, req, tc.projectID, uuid.New(), uuid.New())

	require.Equal(t, 401, res.Code)
}

func TestGetOrganizationScheduled(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	ctx := context.Background()

	orgID := tc.createOrg(t)
	sid := tc.createSchedule(t, "org_list", "single")

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err := tc.store.CreateOrganizationSchedule(ctx, orgID, sid, &futureTime, nil, nil, json.RawMessage(`{"team":"eng"}`))
	require.NoError(t, err)

	limit := oapi.PaginationLimit(20)
	offset := oapi.PaginationOffset(0)
	params := oapi.GetOrganizationScheduledParams{
		Limit:  &limit,
		Offset: &offset,
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(tc.actorCtx)

	tc.controller.GetOrganizationScheduled(res, req, tc.projectID, orgID, params)

	require.Equal(t, 200, res.Code)

	var response oapi.UserScheduledList
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, 1, response.Total)
	require.Len(t, response.Results, 1)
	require.Equal(t, orgID, response.Results[0].UserId)
	require.Equal(t, sid, response.Results[0].ScheduledId)
}

func TestGetOrganizationScheduledEmpty(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	orgID := tc.createOrg(t)

	limit := oapi.PaginationLimit(20)
	offset := oapi.PaginationOffset(0)
	params := oapi.GetOrganizationScheduledParams{
		Limit:  &limit,
		Offset: &offset,
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(tc.actorCtx)

	tc.controller.GetOrganizationScheduled(res, req, tc.projectID, orgID, params)

	require.Equal(t, 200, res.Code)

	var response oapi.UserScheduledList
	err := json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, 0, response.Total)
	require.Empty(t, response.Results)
}

func TestGetOrganizationScheduledOrgNotFound(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)

	limit := oapi.PaginationLimit(20)
	offset := oapi.PaginationOffset(0)
	params := oapi.GetOrganizationScheduledParams{
		Limit:  &limit,
		Offset: &offset,
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(tc.actorCtx)

	tc.controller.GetOrganizationScheduled(res, req, tc.projectID, uuid.New(), params)

	require.Equal(t, 404, res.Code)
}

func TestGetOrganizationScheduledUnauthorized(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	orgID := tc.createOrg(t)

	limit := oapi.PaginationLimit(20)
	offset := oapi.PaginationOffset(0)
	params := oapi.GetOrganizationScheduledParams{
		Limit:  &limit,
		Offset: &offset,
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	tc.controller.GetOrganizationScheduled(res, req, tc.projectID, orgID, params)

	require.Equal(t, 401, res.Code)
}

func TestUpsertOrganizationScheduled(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)

	orgID := tc.createOrg(t)
	sid := tc.createSchedule(t, "org_upsert", "single")

	futureTime := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Microsecond)
	data := json.RawMessage(`{"meeting":"standup"}`)
	body, _ := json.Marshal(oapi.UpsertOrganizationScheduledRequest{
		ScheduledId: &sid,
		ScheduledAt: &futureTime,
		Data:        &data,
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/", bytes.NewReader(body))
	req = req.WithContext(tc.actorCtx)

	tc.controller.UpsertOrganizationScheduled(res, req, tc.projectID, orgID)

	require.Equal(t, 200, res.Code)

	var result oapi.UserScheduled
	err := json.Unmarshal(res.Body.Bytes(), &result)
	require.NoError(t, err)
	require.Equal(t, orgID, result.UserId)
	require.Equal(t, sid, result.ScheduledId)
	require.WithinDuration(t, futureTime, result.ScheduledAt, time.Second)
}

func TestUpsertOrganizationScheduledRecurring(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)

	orgID := tc.createOrg(t)
	sid := tc.createSchedule(t, "org_recurring", "recurring")

	startAt := time.Now().Add(-14 * 24 * time.Hour).UTC().Truncate(time.Microsecond)
	interval := "7 days"
	data := json.RawMessage(`{}`)
	body, _ := json.Marshal(oapi.UpsertOrganizationScheduledRequest{
		ScheduledId: &sid,
		StartAt:     &startAt,
		Interval:    &interval,
		Data:        &data,
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/", bytes.NewReader(body))
	req = req.WithContext(tc.actorCtx)

	tc.controller.UpsertOrganizationScheduled(res, req, tc.projectID, orgID)

	require.Equal(t, 200, res.Code)

	var result oapi.UserScheduled
	err := json.Unmarshal(res.Body.Bytes(), &result)
	require.NoError(t, err)
	require.NotNil(t, result.Interval)
	require.True(t, result.ScheduledAt.After(time.Now()))
}

func TestUpsertOrganizationScheduledMissingScheduledAt(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)

	orgID := tc.createOrg(t)
	sid := tc.createSchedule(t, "org_missing_at", "single")

	data := json.RawMessage(`{}`)
	body, _ := json.Marshal(oapi.UpsertOrganizationScheduledRequest{
		ScheduledId: &sid,
		Data:        &data,
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/", bytes.NewReader(body))
	req = req.WithContext(tc.actorCtx)

	tc.controller.UpsertOrganizationScheduled(res, req, tc.projectID, orgID)

	require.Equal(t, 400, res.Code)
}

func TestUpsertOrganizationScheduledOrgNotFound(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	sid := tc.createSchedule(t, "org_nf", "single")

	futureTime := time.Now().Add(24 * time.Hour).UTC()
	data := json.RawMessage(`{}`)
	body, _ := json.Marshal(oapi.UpsertOrganizationScheduledRequest{
		ScheduledId: &sid,
		ScheduledAt: &futureTime,
		Data:        &data,
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/", bytes.NewReader(body))
	req = req.WithContext(tc.actorCtx)

	tc.controller.UpsertOrganizationScheduled(res, req, tc.projectID, uuid.New())

	require.Equal(t, 404, res.Code)
}

func TestUpsertOrganizationScheduledUnauthorized(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	orgID := tc.createOrg(t)
	sid := tc.createSchedule(t, "org_unauth", "single")

	futureTime := time.Now().Add(24 * time.Hour).UTC()
	data := json.RawMessage(`{}`)
	body, _ := json.Marshal(oapi.UpsertOrganizationScheduledRequest{
		ScheduledId: &sid,
		ScheduledAt: &futureTime,
		Data:        &data,
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/", bytes.NewReader(body))

	tc.controller.UpsertOrganizationScheduled(res, req, tc.projectID, orgID)

	require.Equal(t, 401, res.Code)
}

func TestUpsertOrganizationScheduledIdempotent(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)

	orgID := tc.createOrg(t)
	sid := tc.createSchedule(t, "org_idem", "single")

	time1 := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	data := json.RawMessage(`{}`)
	body1, _ := json.Marshal(oapi.UpsertOrganizationScheduledRequest{
		ScheduledId: &sid,
		ScheduledAt: &time1,
		Data:        &data,
	})

	res1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("PUT", "/", bytes.NewReader(body1))
	req1 = req1.WithContext(tc.actorCtx)
	tc.controller.UpsertOrganizationScheduled(res1, req1, tc.projectID, orgID)
	require.Equal(t, 200, res1.Code)

	var result1 oapi.UserScheduled
	require.NoError(t, json.Unmarshal(res1.Body.Bytes(), &result1))

	time2 := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Microsecond)
	body2, _ := json.Marshal(oapi.UpsertOrganizationScheduledRequest{
		ScheduledId: &sid,
		ScheduledAt: &time2,
		Data:        &data,
	})

	res2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("PUT", "/", bytes.NewReader(body2))
	req2 = req2.WithContext(tc.actorCtx)
	tc.controller.UpsertOrganizationScheduled(res2, req2, tc.projectID, orgID)
	require.Equal(t, 200, res2.Code)

	var result2 oapi.UserScheduled
	require.NoError(t, json.Unmarshal(res2.Body.Bytes(), &result2))

	require.Equal(t, result1.Id, result2.Id, "upsert should return same instance ID")
	require.WithinDuration(t, time2, result2.ScheduledAt, time.Second)
}

func TestDeleteOrganizationScheduled(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	ctx := context.Background()

	orgID := tc.createOrg(t)
	sid := tc.createSchedule(t, "org_del", "single")

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	os, err := tc.store.CreateOrganizationSchedule(ctx, orgID, sid, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/", nil)
	req = req.WithContext(tc.actorCtx)

	tc.controller.DeleteOrganizationScheduled(res, req, tc.projectID, orgID, os.ID)

	require.Equal(t, 204, res.Code)

	_, err = tc.store.GetOrganizationScheduleByID(ctx, os.ID)
	require.Error(t, err)
}

func TestDeleteOrganizationScheduledNotFound(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	orgID := tc.createOrg(t)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/", nil)
	req = req.WithContext(tc.actorCtx)

	tc.controller.DeleteOrganizationScheduled(res, req, tc.projectID, orgID, uuid.New())

	require.Equal(t, 404, res.Code)
}

func TestDeleteOrganizationScheduledWrongOrg(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	ctx := context.Background()

	org1 := tc.createOrg(t)
	org2 := tc.createOrg(t)
	sid := tc.createSchedule(t, "wrong_org_del", "single")

	futureTime := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	os, err := tc.store.CreateOrganizationSchedule(ctx, org1, sid, &futureTime, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/", nil)
	req = req.WithContext(tc.actorCtx)

	tc.controller.DeleteOrganizationScheduled(res, req, tc.projectID, org2, os.ID)

	require.Equal(t, 404, res.Code)
}

func TestDeleteOrganizationScheduledUnauthorized(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	orgID := tc.createOrg(t)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/", nil)

	tc.controller.DeleteOrganizationScheduled(res, req, tc.projectID, orgID, uuid.New())

	require.Equal(t, 401, res.Code)
}

func TestUpdateOrganizationScheduled(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	ctx := context.Background()

	orgID := tc.createOrg(t)
	sid := tc.createSchedule(t, "org_update", "single")

	original := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Microsecond)
	os, err := tc.store.CreateOrganizationSchedule(ctx, orgID, sid, &original, nil, nil, json.RawMessage(`{}`))
	require.NoError(t, err)

	newTime := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Microsecond)
	body, _ := json.Marshal(oapi.UpdateOrganizationScheduledRequest{
		ScheduledAt: &newTime,
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/", bytes.NewReader(body))
	req = req.WithContext(tc.actorCtx)

	tc.controller.UpdateOrganizationScheduled(res, req, tc.projectID, orgID, os.ID)

	require.Equal(t, 200, res.Code)

	var result oapi.UserScheduled
	err = json.Unmarshal(res.Body.Bytes(), &result)
	require.NoError(t, err)
	require.WithinDuration(t, newTime, result.ScheduledAt, time.Second)
}

func TestUpdateOrganizationScheduledUnauthorized(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)

	newTime := time.Now().Add(72 * time.Hour).UTC()
	body, _ := json.Marshal(oapi.UpdateOrganizationScheduledRequest{
		ScheduledAt: &newTime,
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/", bytes.NewReader(body))

	tc.controller.UpdateOrganizationScheduled(res, req, tc.projectID, uuid.New(), uuid.New())

	require.Equal(t, 401, res.Code)
}

func TestGetUserScheduledPagination(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	ctx := context.Background()

	userID := tc.createUser(t)

	for i := 0; i < 5; i++ {
		sid := tc.createSchedule(t, "page_"+uuid.New().String()[:8], "single")
		ft := time.Now().Add(time.Duration(i+1) * 24 * time.Hour).UTC().Truncate(time.Microsecond)
		_, err := tc.store.CreateUserSchedule(ctx, userID, sid, &ft, nil, nil, json.RawMessage(`{}`))
		require.NoError(t, err)
	}

	limit := oapi.PaginationLimit(2)
	offset := oapi.PaginationOffset(0)
	params := oapi.GetUserScheduledParams{
		Limit:  &limit,
		Offset: &offset,
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(tc.actorCtx)

	tc.controller.GetUserScheduled(res, req, tc.projectID, userID, params)

	require.Equal(t, 200, res.Code)

	var response oapi.UserScheduledList
	err := json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, 5, response.Total)
	require.Len(t, response.Results, 2)
	require.Equal(t, 2, response.Limit)
	require.Equal(t, 0, response.Offset)
}

func TestGetOrganizationScheduledPagination(t *testing.T) {
	t.Parallel()

	tc := setupScheduledController(t)
	ctx := context.Background()

	orgID := tc.createOrg(t)

	for i := 0; i < 5; i++ {
		sid := tc.createSchedule(t, "org_page_"+uuid.New().String()[:8], "single")
		ft := time.Now().Add(time.Duration(i+1) * 24 * time.Hour).UTC().Truncate(time.Microsecond)
		_, err := tc.store.CreateOrganizationSchedule(ctx, orgID, sid, &ft, nil, nil, json.RawMessage(`{}`))
		require.NoError(t, err)
	}

	limit := oapi.PaginationLimit(3)
	offset := oapi.PaginationOffset(0)
	params := oapi.GetOrganizationScheduledParams{
		Limit:  &limit,
		Offset: &offset,
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req = req.WithContext(tc.actorCtx)

	tc.controller.GetOrganizationScheduled(res, req, tc.projectID, orgID, params)

	require.Equal(t, 200, res.Code)

	var response oapi.UserScheduledList
	err := json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, 5, response.Total)
	require.Len(t, response.Results, 3)
}

func TestUserScheduleToOAPI(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Microsecond)
	userID := uuid.New()
	scheduleID := uuid.New()
	instanceID := uuid.New()

	us := subjects.UserSchedule{
		ID:          instanceID,
		UserID:      userID,
		ScheduleID:  scheduleID,
		ScheduledAt: &now,
		Data:        json.RawMessage(`{"key":"value"}`),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	result := userScheduleToOAPI(us)
	require.Equal(t, instanceID, result.Id)
	require.Equal(t, userID, result.UserId)
	require.Equal(t, scheduleID, result.ScheduledId)
	require.Equal(t, now, result.ScheduledAt)
	require.JSONEq(t, `{"key":"value"}`, string(result.Data))
}

func TestUserScheduleToOAPIRecurringFallback(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Microsecond)
	startAt := now.Add(-14 * 24 * time.Hour)
	interval := "7 days"

	us := subjects.UserSchedule{
		ID:         uuid.New(),
		UserID:     uuid.New(),
		ScheduleID: uuid.New(),
		StartAt:    &startAt,
		Interval:   &interval,
		Data:       json.RawMessage(`{}`),
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	result := userScheduleToOAPI(us)
	require.Equal(t, startAt, result.ScheduledAt, "should fall back to start_at when scheduled_at is nil")
	require.NotNil(t, result.Interval)
	require.NotNil(t, result.StartAt)
}

func TestOrgScheduleToOAPI(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Microsecond)
	orgID := uuid.New()
	scheduleID := uuid.New()

	os := subjects.OrganizationSchedule{
		ID:             uuid.New(),
		OrganizationID: orgID,
		ScheduleID:     scheduleID,
		ScheduledAt:    &now,
		Data:           json.RawMessage(`{"org":"data"}`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	result := orgScheduleToOAPI(os)
	require.Equal(t, orgID, result.UserId, "UserId should carry OrganizationID")
	require.Equal(t, scheduleID, result.ScheduledId)
	require.Equal(t, now, result.ScheduledAt)
}

func TestOrgScheduleToOAPIRecurringFallback(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Microsecond)
	startAt := now.Add(-7 * 24 * time.Hour)
	interval := "1 month"

	os := subjects.OrganizationSchedule{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		ScheduleID:     uuid.New(),
		StartAt:        &startAt,
		Interval:       &interval,
		Data:           json.RawMessage(`{}`),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	result := orgScheduleToOAPI(os)
	require.Equal(t, startAt, result.ScheduledAt, "should fall back to start_at when scheduled_at is nil")
}
