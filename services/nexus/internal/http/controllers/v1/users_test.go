package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lunogram/platform/pkg/claim"
	"github.com/lunogram/platform/pkg/container"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/lunogram/platform/services/nexus/oapi"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func setupUsersController(t *testing.T) (*UsersController, uuid.UUID) {
	t.Helper()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(config.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, config.Store)
	require.NoError(t, err)

	orgsStore := store.NewOrganizationsStore(db)
	orgID, err := orgsStore.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projectsStore := store.NewProjectsStore(db)
	projectID, err := projectsStore.CreateProject(ctx, store.Project{
		OrganizationID: &orgID,
		Name:           DefaultProject.Name,
		Timezone:       DefaultProject.Timezone,
		Locale:         DefaultProject.Locale,
	})
	require.NoError(t, err)

	controller := NewUsersController(logger, db)
	return controller, projectID
}

func validSession() claim.Session {
	return claim.Session{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: uuid.New().String(),
		},
	}
}

func TestListUsers(t *testing.T) {
	t.Parallel()
	controller, projectID := setupUsersController(t)
	ctx := context.Background()

	usersStore := controller.store.UsersStore
	for i := 0; i < 5; i++ {
		_, err := usersStore.CreateUser(ctx, store.User{
			ProjectID:   projectID,
			AnonymousID: ptr(uuid.New().String()),
			Data:        json.RawMessage(`{}`),
		})
		require.NoError(t, err)
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String()+"/users", nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

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
	controller, projectID := setupUsersController(t)

	body := oapi.IdentifyUser{
		ExternalId: ptr("user_new_123"),
		Email:      ptr("new@example.com"),
	}

	bodyBytes, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/admin/projects/"+projectID.String()+"/users", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

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
	controller, projectID := setupUsersController(t)
	ctx := context.Background()

	usersStore := controller.store.UsersStore
	userID, err := usersStore.CreateUser(ctx, store.User{
		ProjectID:   projectID,
		AnonymousID: ptr("anon_get"),
		Email:       ptr("get@example.com"),
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String()+"/users/"+userID.String(), nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

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
	controller, projectID := setupUsersController(t)
	ctx := context.Background()

	usersStore := controller.store.UsersStore
	userID, err := usersStore.CreateUser(ctx, store.User{
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
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	controller.UpdateUser(res, req, projectID, userID)

	require.Equal(t, 200, res.Code)

	var user oapi.User
	err = json.Unmarshal(res.Body.Bytes(), &user)
	require.NoError(t, err)
	require.Equal(t, "updated@example.com", string(*user.Email))

	var data map[string]interface{}
	err = json.Unmarshal(user.Data, &data)
	require.NoError(t, err)
	require.Contains(t, data, "old", "should preserve existing keys")
	require.Contains(t, data, "new", "should add new keys")
}

func TestDeleteUser(t *testing.T) {
	t.Parallel()
	controller, projectID := setupUsersController(t)
	ctx := context.Background()

	usersStore := controller.store.UsersStore
	userID, err := usersStore.CreateUser(ctx, store.User{
		ProjectID:   projectID,
		AnonymousID: ptr("anon_delete"),
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/api/admin/projects/"+projectID.String()+"/users/"+userID.String(), nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	controller.DeleteUser(res, req, projectID, userID)

	require.Equal(t, 204, res.Code)

	_, err = usersStore.GetUser(context.Background(), projectID, userID)
	require.Error(t, err, "user should be deleted")
}

func TestVersionIncrementsOnUpdate(t *testing.T) {
	t.Parallel()
	controller, projectID := setupUsersController(t)
	ctx := context.Background()

	usersStore := controller.store.UsersStore
	userID, err := usersStore.CreateUser(ctx, store.User{
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
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	controller.UpdateUser(res, req, projectID, userID)
	require.Equal(t, 200, res.Code)

	var updatedUser oapi.User
	err = json.Unmarshal(res.Body.Bytes(), &updatedUser)
	require.NoError(t, err)
	require.Equal(t, initialVersion+1, updatedUser.Version, "version should auto-increment via trigger")
}

func TestGetUserEvents(t *testing.T) {
	t.Parallel()
	controller, projectID := setupUsersController(t)
	ctx := context.Background()

	usersStore := controller.store.UsersStore
	userID, err := usersStore.CreateUser(ctx, store.User{
		ProjectID:   projectID,
		AnonymousID: ptr("anon_events"),
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		_, err := usersStore.CreateUserEvent(ctx, store.UserEvent{
			ProjectID: projectID,
			UserID:    userID,
			Name:      "page_viewed",
			Data:      json.RawMessage(`{"page":"/home"}`),
		})
		require.NoError(t, err)
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String()+"/users/"+userID.String()+"/events", nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

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
	controller, projectID := setupUsersController(t)

	nonExistentUserID := uuid.New()

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String()+"/users/"+nonExistentUserID.String()+"/events", nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	controller.GetUserEvents(res, req, projectID, nonExistentUserID, oapi.GetUserEventsParams{})

	require.Equal(t, 404, res.Code)
}

func TestGetUserSubscriptions(t *testing.T) {
	t.Parallel()
	controller, projectID := setupUsersController(t)
	ctx := context.Background()

	usersStore := controller.store.UsersStore
	userID, err := usersStore.CreateUser(ctx, store.User{
		ProjectID:   projectID,
		AnonymousID: ptr("anon_subscriptions"),
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	subscriptionsStore := controller.store.SubscriptionsStore
	subscriptionID1, err := subscriptionsStore.CreateSubscription(ctx, store.Subscription{
		ProjectID: projectID,
		Name:      "Newsletter",
		Channel:   "email",
		IsPublic:  true,
	})
	require.NoError(t, err)

	_, err = subscriptionsStore.CreateSubscription(ctx, store.Subscription{
		ProjectID: projectID,
		Name:      "SMS Updates",
		Channel:   "sms",
		IsPublic:  true,
	})
	require.NoError(t, err)

	err = subscriptionsStore.ToggleSubscription(ctx, userID, subscriptionID1, "unsubscribed")
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String()+"/users/"+userID.String()+"/subscriptions", nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

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
	controller, projectID := setupUsersController(t)
	ctx := context.Background()

	usersStore := controller.store.UsersStore
	userID, err := usersStore.CreateUser(ctx, store.User{
		ProjectID:   projectID,
		AnonymousID: ptr("anon_update_subs"),
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	subscriptionsStore := controller.store.SubscriptionsStore
	subscriptionID, err := subscriptionsStore.CreateSubscription(ctx, store.Subscription{
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
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

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
	controller, projectID := setupUsersController(t)
	ctx := context.Background()

	usersStore := controller.store.UsersStore
	userID, err := usersStore.CreateUser(ctx, store.User{
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
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	controller.UpdateUserSubscriptions(res, req, projectID, userID)

	require.Equal(t, 404, res.Code)
}

func TestGetUserJourneys(t *testing.T) {
	t.Parallel()
	controller, projectID := setupUsersController(t)
	ctx := context.Background()

	usersStore := controller.store.UsersStore
	userID, err := usersStore.CreateUser(ctx, store.User{
		ProjectID:   projectID,
		AnonymousID: ptr("anon_journeys"),
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	journeysStore := controller.store.JourneysStore
	status := "active"
	journeyID, err := journeysStore.CreateJourney(ctx, store.Journey{
		ProjectID: projectID,
		Name:      "Onboarding Flow",
		Status:    &status,
	})
	require.NoError(t, err)

	entranceID, err := journeysStore.CreateUserJourneyStep(ctx, userID, journeyID, "entrance")
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String()+"/users/"+userID.String()+"/journeys", nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	controller.GetUserJourneys(res, req, projectID, userID, oapi.GetUserJourneysParams{})

	require.Equal(t, 200, res.Code)

	var response oapi.UserJourneyList
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, 1, response.Total)
	require.Len(t, response.Results, 1)
	require.Equal(t, entranceID, response.Results[0].Id)
	require.NotNil(t, response.Results[0].Journey)
	require.Equal(t, "Onboarding Flow", response.Results[0].Journey.Name)
}

func TestGetUserJourneysPagination(t *testing.T) {
	t.Parallel()
	controller, projectID := setupUsersController(t)
	ctx := context.Background()

	usersStore := controller.store.UsersStore
	userID, err := usersStore.CreateUser(ctx, store.User{
		ProjectID:   projectID,
		AnonymousID: ptr("anon_journeys_page"),
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	journeysStore := controller.store.JourneysStore
	status := "active"
	journeyID, err := journeysStore.CreateJourney(ctx, store.Journey{
		ProjectID: projectID,
		Name:      "Test Journey",
		Status:    &status,
	})
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		_, err = journeysStore.CreateUserJourneyStep(ctx, userID, journeyID, "entrance")
		require.NoError(t, err)
	}

	limit := oapi.PaginationLimit(2)
	offset := oapi.PaginationOffset(1)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String()+"/users/"+userID.String()+"/journeys", nil)
	req = req.WithContext(claim.WithSession(req.Context(), validSession()))

	controller.GetUserJourneys(res, req, projectID, userID, oapi.GetUserJourneysParams{
		Limit:  &limit,
		Offset: &offset,
	})

	require.Equal(t, 200, res.Code)

	var response oapi.UserJourneyList
	err = json.Unmarshal(res.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, 5, response.Total)
	require.Len(t, response.Results, 2)
	require.Equal(t, 2, response.Limit)
	require.Equal(t, 1, response.Offset)
}
