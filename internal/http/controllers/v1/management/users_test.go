package v1

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/consumer"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

//go:embed test/users/valid.csv
var validImportUsersCSV string

//go:embed test/users/no-external-id.csv
var noExternalIDImportUsersCSV string

//go:embed test/users/out-of-order.csv
var outOfOrderImportUsersCSV string

func TNewUsersController(t *testing.T) (*UsersController, uuid.UUID, context.Context) {
	t.Helper()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	gracefulCtx := graceful.NewContext(ctx)
	mgmtDB, usrsDB, jrnyDB := teststore.RunPostgreSQL(t)
	cfg := config.Node{
		Nats: config.Nats{
			URL: container.RunNATS(t),
		},
	}

	jet, err := pubsub.New(gracefulCtx, cfg)
	require.NoError(t, err)

	err = consumer.Bootstrap(gracefulCtx, logger, jet, "")
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, "")

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

	mgmt := management.NewState(mgmtDB)
	controller := NewUsersController(logger, pub, usrsDB, jrnyDB, mgmt, 32<<20, engine)
	return controller, projectID, actorCtx
}

func TestListUsers(t *testing.T) {
	t.Parallel()

	controller, projectID, actorCtx := TNewUsersController(t)
	ctx := context.Background()

	usersStore := controller.users.UsersStore
	for i := 0; i < 5; i++ {
		_, err := usersStore.CreateUser(ctx, subjects.User{
			ProjectID:   projectID,
			AnonymousID: ptr(uuid.New().String()),
			Data:        json.RawMessage(`{}`),
		})
		require.NoError(t, err)
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String()+"/users", nil)
	req = req.WithContext(actorCtx)

	controller.ListUsers(res, req, projectID, oapi.ListUsersParams{})

	require.Equal(t, 200, res.Code)

	var response oapi.UserList
	err := json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, 5, response.Total)
	require.Len(t, response.Results, 5)
}

func TestIdentifyUser(t *testing.T) {
	t.Parallel()

	controller, projectID, actorCtx := TNewUsersController(t)

	body := oapi.IdentifyUser{
		ExternalId: ptr("user_new_123"),
		Email:      ptr("new@example.com"),
	}

	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/admin/projects/"+projectID.String()+"/users", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(actorCtx)

	controller.IdentifyUser(res, req, projectID)

	require.Equal(t, 200, res.Code)

	var user oapi.User
	err = json.Unmarshal(res.Body.Bytes(), &user)
	require.NoError(t, err)
	require.Equal(t, "user_new_123", *user.ExternalId)
	require.Equal(t, "new@example.com", string(*user.Email))
}

func TestGetUser(t *testing.T) {
	t.Parallel()

	controller, projectID, actorCtx := TNewUsersController(t)
	ctx := context.Background()

	usersStore := controller.users.UsersStore
	userID, err := usersStore.CreateUser(ctx, subjects.User{
		ProjectID:   projectID,
		AnonymousID: ptr("anon_get"),
		Email:       ptr("get@example.com"),
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String()+"/users/"+userID.String(), nil)
	req = req.WithContext(actorCtx)

	controller.GetUser(res, req, projectID, userID)

	require.Equal(t, 200, res.Code)

	var user oapi.User
	err = json.Unmarshal(res.Body.Bytes(), &user)
	require.NoError(t, err)
	require.Equal(t, userID, user.Id)
	require.Equal(t, "anon_get", user.AnonymousId)
}

func TestUpdateUser(t *testing.T) {
	t.Parallel()

	controller, projectID, actorCtx := TNewUsersController(t)
	ctx := context.Background()

	usersStore := controller.users.UsersStore
	userID, err := usersStore.CreateUser(ctx, subjects.User{
		ProjectID:   projectID,
		AnonymousID: ptr("anon_update"),
		Email:       ptr("old@example.com"),
		Data:        json.RawMessage(`{"old":"value"}`),
	})
	require.NoError(t, err)

	updateBody := oapi.UpdateUser{
		Email: ptr("updated@example.com"),
		Data:  ptr(json.RawMessage(`{"new":"field"}`)),
	}

	bodyBytes, err := json.Marshal(updateBody)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/admin/projects/"+projectID.String()+"/users/"+userID.String(), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(actorCtx)

	controller.UpdateUser(res, req, projectID, userID)

	require.Equal(t, 200, res.Code)

	var user oapi.User
	err = json.Unmarshal(res.Body.Bytes(), &user)
	require.NoError(t, err)
	require.Equal(t, "updated@example.com", string(*user.Email))

	var data map[string]any
	err = json.Unmarshal(user.Data, &data)
	require.NoError(t, err)
	require.Contains(t, data, "old", "should preserve existing keys")
	require.Contains(t, data, "new", "should add new keys")
}

func TestDeleteUser(t *testing.T) {
	t.Parallel()

	controller, projectID, actorCtx := TNewUsersController(t)
	ctx := context.Background()

	usersStore := controller.users.UsersStore
	userID, err := usersStore.CreateUser(ctx, subjects.User{
		ProjectID:   projectID,
		AnonymousID: ptr("anon_delete"),
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/admin/projects/"+projectID.String()+"/users/"+userID.String(), nil)
	req = req.WithContext(actorCtx)

	controller.DeleteUser(res, req, projectID, userID)

	require.Equal(t, 204, res.Code)

	_, err = usersStore.GetUser(context.Background(), projectID, userID)
	require.Error(t, err, "user should be deleted")
}

func TestVersionIncrementsOnUpdate(t *testing.T) {
	t.Parallel()

	controller, projectID, actorCtx := TNewUsersController(t)
	ctx := context.Background()

	usersStore := controller.users.UsersStore
	userID, err := usersStore.CreateUser(ctx, subjects.User{
		ProjectID:   projectID,
		AnonymousID: ptr("anon_version"),
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	user, err := usersStore.GetUser(ctx, projectID, userID)
	require.NoError(t, err)
	initialVersion := user.Version

	bodyBytes, err := json.Marshal(oapi.UpdateUser{
		Email: ptr("version@example.com"),
	})
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/admin/projects/"+projectID.String()+"/users/"+userID.String(), bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(actorCtx)

	controller.UpdateUser(res, req, projectID, userID)
	require.Equal(t, 200, res.Code)

	var updatedUser oapi.User
	err = json.Unmarshal(res.Body.Bytes(), &updatedUser)
	require.NoError(t, err)
	require.Equal(t, initialVersion+1, updatedUser.Version, "version should auto-increment via trigger")
}

func TestGetUserEvents(t *testing.T) {
	t.Parallel()

	controller, projectID, actorCtx := TNewUsersController(t)
	ctx := context.Background()

	usersStore := controller.users.UsersStore
	userID, err := usersStore.CreateUser(ctx, subjects.User{
		ProjectID:   projectID,
		AnonymousID: ptr("anon_events"),
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		_, err := usersStore.CreateUserEvent(ctx, subjects.UserEvent{
			ProjectID: projectID,
			UserID:    userID,
			Name:      "page_viewed",
			Data:      json.RawMessage(`{"page":"/home"}`),
		})
		require.NoError(t, err)
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String()+"/users/"+userID.String()+"/events", nil)
	req = req.WithContext(actorCtx)

	controller.GetUserEvents(res, req, projectID, userID, oapi.GetUserEventsParams{})

	require.Equal(t, 200, res.Code)

	var response oapi.UserEventList
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, 3, response.Total)
	require.Len(t, response.Results, 3)
	require.Equal(t, "page_viewed", response.Results[0].Name)
}

func TestGetUserEventsNotFound(t *testing.T) {
	t.Parallel()

	controller, projectID, actorCtx := TNewUsersController(t)

	nonExistentUserID := uuid.New()

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String()+"/users/"+nonExistentUserID.String()+"/events", nil)
	req = req.WithContext(actorCtx)

	controller.GetUserEvents(res, req, projectID, nonExistentUserID, oapi.GetUserEventsParams{})

	require.Equal(t, 404, res.Code)
}

func TestGetUserSubscriptions(t *testing.T) {
	t.Parallel()

	controller, projectID, actorCtx := TNewUsersController(t)
	ctx := context.Background()

	usersStore := controller.users.UsersStore
	userID, err := usersStore.CreateUser(ctx, subjects.User{
		ProjectID:   projectID,
		AnonymousID: ptr("anon_subscriptions"),
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	subscriptionsStore := controller.mgmt.SubscriptionsStore
	subscriptionID1, err := subscriptionsStore.CreateSubscription(ctx, management.Subscription{
		ProjectID: projectID,
		Name:      "Newsletter",
		Channel:   "email",
		IsPublic:  true,
	})
	require.NoError(t, err)

	_, err = subscriptionsStore.CreateSubscription(ctx, management.Subscription{
		ProjectID: projectID,
		Name:      "SMS Updates",
		Channel:   "sms",
		IsPublic:  true,
	})
	require.NoError(t, err)

	err = subscriptionsStore.Unsubscribe(ctx, userID, subscriptionID1)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String()+"/users/"+userID.String()+"/subscriptions", nil)
	req = req.WithContext(actorCtx)

	controller.GetUserSubscriptions(res, req, projectID, userID, oapi.GetUserSubscriptionsParams{})

	require.Equal(t, 200, res.Code)

	var response oapi.UserSubscriptionList
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, 2, response.Total)
	require.Len(t, response.Results, 2)

	for _, sub := range response.Results {
		if sub.SubscriptionId == subscriptionID1 {
			require.Equal(t, "unsubscribed", string(sub.State))
		} else {
			require.Equal(t, "subscribed", string(sub.State))
		}
	}
}

func TestUpdateUserSubscriptions(t *testing.T) {
	t.Parallel()

	controller, projectID, actorCtx := TNewUsersController(t)
	ctx := context.Background()

	usersStore := controller.users.UsersStore
	userID, err := usersStore.CreateUser(ctx, subjects.User{
		ProjectID:   projectID,
		AnonymousID: ptr("anon_update_subs"),
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	subscriptionsStore := controller.mgmt.SubscriptionsStore
	subscriptionID, err := subscriptionsStore.CreateSubscription(ctx, management.Subscription{
		ProjectID: projectID,
		Name:      "Marketing",
		Channel:   "email",
		IsPublic:  true,
	})
	require.NoError(t, err)

	updateBody := oapi.UpdateUserSubscriptions{
		{
			SubscriptionId: subscriptionID,
			State:          "unsubscribed",
		},
	}

	bodyBytes, err := json.Marshal(updateBody)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/admin/projects/"+projectID.String()+"/users/"+userID.String()+"/subscriptions", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(actorCtx)

	controller.UpdateUserSubscriptions(res, req, projectID, userID)

	require.Equal(t, 200, res.Code)

	// Verify the subscription state changed to unsubscribed
	subs, _, err := subscriptionsStore.GetUserSubscriptions(ctx, projectID, userID, store.Pagination{Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Len(t, subs, 1)
	require.Equal(t, "unsubscribed", subs[0].State)
}

func TestUpdateUserSubscriptionsNotFound(t *testing.T) {
	t.Parallel()

	controller, projectID, actorCtx := TNewUsersController(t)
	ctx := context.Background()

	usersStore := controller.users.UsersStore
	userID, err := usersStore.CreateUser(ctx, subjects.User{
		ProjectID:   projectID,
		AnonymousID: ptr("anon_sub_not_found"),
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	nonExistentSubID := uuid.New()
	updateBody := oapi.UpdateUserSubscriptions{
		{
			SubscriptionId: nonExistentSubID,
			State:          "unsubscribed",
		},
	}

	bodyBytes, err := json.Marshal(updateBody)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/api/admin/projects/"+projectID.String()+"/users/"+userID.String()+"/subscriptions", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(actorCtx)

	controller.UpdateUserSubscriptions(res, req, projectID, userID)

	require.Equal(t, 404, res.Code)
}

func TestGetUserJourneys(t *testing.T) {
	t.Parallel()

	controller, projectID, actorCtx := TNewUsersController(t)
	ctx := context.Background()

	usersStore := controller.users.UsersStore
	userID, err := usersStore.CreateUser(ctx, subjects.User{
		ProjectID:   projectID,
		AnonymousID: ptr("anon_journeys"),
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	journeysStore := controller.journey.JourneysStore
	journeyID, err := journeysStore.CreateJourney(ctx, journey.Journey{
		ProjectID: projectID,
		Name:      "Onboarding Flow",
	})
	require.NoError(t, err)

	// Create initial version and link to journey
	versionID, err := journeysStore.CreateJourneyVersion(ctx, journeyID, "draft")
	require.NoError(t, err)
	err = journeysStore.UpdateJourney(ctx, projectID, journeyID, journey.JourneyUpdate{VersionID: &versionID})
	require.NoError(t, err)

	// Create a step so we have a step ID
	stepMap := oapi.JourneyStepMap{
		"entrance-1": {
			Type: "entrance",
			X:    100,
			Y:    200,
		},
	}
	_, err = journeysStore.SetJourneySteps(ctx, versionID, stepMap)
	require.NoError(t, err)

	// Create user state
	journeyEntryID := uuid.New()
	_, err = journeysStore.CreateUserJourneyState(ctx, journey.JourneyUserState{
		JourneyID:      journeyID,
		JourneyEntryID: journeyEntryID,
		UserID:         userID,
		ExternalStepID: "entrance-1",
	})
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String()+"/users/"+userID.String()+"/journeys", nil)
	req = req.WithContext(actorCtx)

	controller.GetUserJourneys(res, req, projectID, userID, oapi.GetUserJourneysParams{})

	require.Equal(t, 200, res.Code)

	var response oapi.UserJourneyList
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	// Should have 1 result since user state was created
	require.Equal(t, 1, response.Total)
	require.Len(t, response.Results, 1)
	require.NotNil(t, response.Results[0].Journey)
	require.Equal(t, "Onboarding Flow", response.Results[0].Journey.Name)
}

func TestGetUserJourneysPagination(t *testing.T) {
	t.Parallel()

	controller, projectID, actorCtx := TNewUsersController(t)
	ctx := context.Background()

	usersStore := controller.users.UsersStore
	userID, err := usersStore.CreateUser(ctx, subjects.User{
		ProjectID:   projectID,
		AnonymousID: ptr("anon_journeys_page"),
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	journeysStore := controller.journey.JourneysStore
	journeyID, err := journeysStore.CreateJourney(ctx, journey.Journey{
		ProjectID: projectID,
		Name:      "Test Journey",
	})
	require.NoError(t, err)

	// Create initial version and link to journey
	versionID, err := journeysStore.CreateJourneyVersion(ctx, journeyID, "draft")
	require.NoError(t, err)
	err = journeysStore.UpdateJourney(ctx, projectID, journeyID, journey.JourneyUpdate{VersionID: &versionID})
	require.NoError(t, err)

	// Create a step so we have a step ID
	stepMap := oapi.JourneyStepMap{
		"entrance-1": {
			Type: "entrance",
			X:    100,
			Y:    200,
		},
	}
	_, err = journeysStore.SetJourneySteps(ctx, versionID, stepMap)
	require.NoError(t, err)

	// Create user state
	journeyEntryID := uuid.New()
	_, err = journeysStore.CreateUserJourneyState(ctx, journey.JourneyUserState{
		JourneyID:      journeyID,
		JourneyEntryID: journeyEntryID,
		UserID:         userID,
		ExternalStepID: "entrance-1",
	})
	require.NoError(t, err)

	limit := oapi.PaginationLimit(2)
	offset := oapi.PaginationOffset(1)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String()+"/users/"+userID.String()+"/journeys", nil)
	req = req.WithContext(actorCtx)

	controller.GetUserJourneys(res, req, projectID, userID, oapi.GetUserJourneysParams{
		Limit:  &limit,
		Offset: &offset,
	})

	require.Equal(t, 200, res.Code)

	var response oapi.UserJourneyList
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	// With offset=1 and only 1 total item, we get 0 results (and total shows 0 because COUNT(*) OVER() returns nothing when no rows)
	require.Equal(t, 0, response.Total)
	require.Len(t, response.Results, 0)
	require.Equal(t, 2, response.Limit)
	require.Equal(t, 1, response.Offset)
}

func TestListUserSchemas(t *testing.T) {
	t.Parallel()

	controller, projectID, actorCtx := TNewUsersController(t)
	ctx := context.Background()

	usersStore := controller.users.UsersStore

	paths := rules.Paths{
		{Path: ".email", Type: "string"},
		{Path: ".age", Type: "number"},
		{Path: ".plan", Type: "string"},
		{Path: ".preferences", Type: "object"},
		{Path: ".preferences.notifications", Type: "boolean"},
	}
	err := usersStore.UpsertUserSchema(ctx, projectID, paths)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String()+"/user-schemas", nil)
	req = req.WithContext(actorCtx)

	controller.ListUserSchemas(res, req, projectID)

	require.Equal(t, 200, res.Code)

	var response struct {
		Results []oapi.SchemaPath `json:"results"`
	}
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)

	// 7 well-known direct columns + 5 discovered data properties
	require.Len(t, response.Results, 12)

	pathMap := make(map[string][]string)
	for _, schema := range response.Results {
		pathMap[schema.Path] = schema.Types
	}

	// Well-known user direct columns
	require.Contains(t, pathMap, ".email")
	require.Contains(t, pathMap[".email"], "string")
	require.Contains(t, pathMap, ".phone")
	require.Contains(t, pathMap[".phone"], "string")
	require.Contains(t, pathMap, ".locale")
	require.Contains(t, pathMap[".locale"], "string")
	require.Contains(t, pathMap, ".timezone")
	require.Contains(t, pathMap[".timezone"], "string")
	require.Contains(t, pathMap, ".external_id")
	require.Contains(t, pathMap[".external_id"], "string")
	require.Contains(t, pathMap, ".anonymous_id")
	require.Contains(t, pathMap[".anonymous_id"], "string")
	require.Contains(t, pathMap, ".created_at")
	require.Contains(t, pathMap[".created_at"], "date")

	// Discovered data properties should have .data prefix
	require.Contains(t, pathMap[".data.email"], "string")
	require.Contains(t, pathMap[".data.age"], "number")
	require.Contains(t, pathMap[".data.plan"], "string")
	require.Contains(t, pathMap[".data.preferences"], "object")
	require.Contains(t, pathMap[".data.preferences.notifications"], "boolean")
}

func TestListUserSchemasEmpty(t *testing.T) {
	t.Parallel()

	controller, projectID, actorCtx := TNewUsersController(t)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String()+"/users/schema", nil)
	req = req.WithContext(actorCtx)

	controller.ListUserSchemas(res, req, projectID)

	require.Equal(t, 200, res.Code)

	var response struct {
		Results []oapi.SchemaPath `json:"results"`
	}
	err := json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)

	// Even with no discovered schemas, well-known direct columns should be present
	require.Len(t, response.Results, 7)

	pathMap := make(map[string][]string)
	for _, schema := range response.Results {
		pathMap[schema.Path] = schema.Types
	}

	require.Contains(t, pathMap, ".email")
	require.Contains(t, pathMap, ".phone")
	require.Contains(t, pathMap, ".locale")
	require.Contains(t, pathMap, ".timezone")
	require.Contains(t, pathMap, ".external_id")
	require.Contains(t, pathMap, ".anonymous_id")
	require.Contains(t, pathMap, ".created_at")
}

func TestListUserSchemasUnauthorized(t *testing.T) {
	t.Parallel()

	controller, projectID, _ := TNewUsersController(t)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String()+"/users/schema", nil)

	controller.ListUserSchemas(res, req, projectID)

	require.Equal(t, 401, res.Code)
}

func TestListUserSchemasWithMultipleTypes(t *testing.T) {
	t.Parallel()

	controller, projectID, actorCtx := TNewUsersController(t)
	ctx := context.Background()

	usersStore := controller.users.UsersStore

	// Insert the same path with different types to simulate real-world scenarios
	// where user data fields might be sent with different types across different users
	paths := rules.Paths{
		{Path: ".age", Type: "number"},
		{Path: ".age", Type: "string"}, // age sent as "25" or 25
		{Path: ".is_active", Type: "boolean"},
		{Path: ".is_active", Type: "string"}, // boolean sent as "true" or true
		{Path: ".tags", Type: "array"},
		{Path: ".tags", Type: "string"}, // tags sent as ["a","b"] or "a,b"
		{Path: ".metadata", Type: "object"},
		{Path: ".name", Type: "string"},
	}
	err := usersStore.UpsertUserSchema(ctx, projectID, paths)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String()+"/users/schema", nil)
	req = req.WithContext(actorCtx)

	controller.ListUserSchemas(res, req, projectID)

	require.Equal(t, 200, res.Code)

	var response struct {
		Results []oapi.SchemaPath `json:"results"`
	}
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)

	// 7 well-known direct columns + 5 discovered data properties
	require.Len(t, response.Results, 12)

	pathMap := make(map[string][]string)
	for _, schema := range response.Results {
		pathMap[schema.Path] = schema.Types
	}

	// Verify .data.age has both number and string types
	require.Contains(t, pathMap, ".data.age")
	require.Len(t, pathMap[".data.age"], 2)
	require.Contains(t, pathMap[".data.age"], "number")
	require.Contains(t, pathMap[".data.age"], "string")

	// Verify .data.is_active has both boolean and string types
	require.Contains(t, pathMap, ".data.is_active")
	require.Len(t, pathMap[".data.is_active"], 2)
	require.Contains(t, pathMap[".data.is_active"], "boolean")
	require.Contains(t, pathMap[".data.is_active"], "string")

	// Verify .data.tags has both array and string types
	require.Contains(t, pathMap, ".data.tags")
	require.Len(t, pathMap[".data.tags"], 2)
	require.Contains(t, pathMap[".data.tags"], "array")
	require.Contains(t, pathMap[".data.tags"], "string")

	// Verify .data.metadata has only object type
	require.Contains(t, pathMap, ".data.metadata")
	require.Len(t, pathMap[".data.metadata"], 1)
	require.Contains(t, pathMap[".data.metadata"], "object")

	// Verify .data.name has only string type
	require.Contains(t, pathMap, ".data.name")
	require.Len(t, pathMap[".data.name"], 1)
	require.Contains(t, pathMap[".data.name"], "string")
}

func TestImportUsers(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)

	type test struct {
		csv   string
		code  int
		users int
	}

	tests := map[string]test{
		"successful-import": {
			csv:   validImportUsersCSV,
			code:  204,
			users: 3,
		},
		"missing-external-id-column": {
			csv:   noExternalIDImportUsersCSV,
			code:  400,
			users: 0,
		},
		"out-of-order-columns": {
			csv:   outOfOrderImportUsersCSV,
			code:  204,
			users: 3,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()
			gracefulCtx := graceful.NewContext(ctx)
			mgmtDB, usrsDB, jrnyDB := teststore.RunPostgreSQL(t)
			cfg := config.Node{
				Nats: config.Nats{
					URL: container.RunNATS(t),
				},
			}

			jet, err := pubsub.New(gracefulCtx, cfg)
			require.NoError(t, err)

			err = consumer.Bootstrap(gracefulCtx, logger, jet, "")
			require.NoError(t, err)

			pub := pubsub.NewPublisher(jet, "")

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

			usersStore := subjects.NewUsersStore(usrsDB)
			mgmt := management.NewState(mgmtDB)
			controller := NewUsersController(logger, pub, usrsDB, jrnyDB, mgmt, 32<<20, engine)

			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			header := textproto.MIMEHeader{}
			header.Set("Content-Disposition", `form-data; name="file"; filename="users.csv"`)
			header.Set("Content-Type", "text/csv")

			part, err := writer.CreatePart(header)
			require.NoError(t, err)

			_, err = part.Write([]byte(test.csv))
			require.NoError(t, err)

			err = writer.Close()
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", fmt.Sprintf("/api/admin/projects/%s/users/import", projectID), body)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			req = req.WithContext(actorCtx)

			controller.ImportUsers(res, req, projectID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			users, total, err := usersStore.ListUsers(ctx, projectID, store.Pagination{Limit: 100, Offset: 0}, "")
			require.NoError(t, err)
			require.Equal(t, test.users, total, "expected %d users in total", test.users)
			require.Len(t, users, test.users, "expected %d users to be returned", test.users)
		})
	}
}
