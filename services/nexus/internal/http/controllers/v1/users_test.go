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
	controller, projectID := setupUsersController(t)
	ctx := context.Background()

	usersStore := controller.store.UsersStore
	for i := 0; i < 5; i++ {
		_, err := usersStore.CreateUser(ctx, store.User{
			ProjectID:   projectID,
			AnonymousID: uuid.New().String(),
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
	controller, projectID := setupUsersController(t)
	ctx := context.Background()

	usersStore := controller.store.UsersStore
	userID, err := usersStore.CreateUser(ctx, store.User{
		ProjectID:   projectID,
		AnonymousID: "anon_get",
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
	controller, projectID := setupUsersController(t)
	ctx := context.Background()

	usersStore := controller.store.UsersStore
	userID, err := usersStore.CreateUser(ctx, store.User{
		ProjectID:   projectID,
		AnonymousID: "anon_update",
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
	controller, projectID := setupUsersController(t)
	ctx := context.Background()

	usersStore := controller.store.UsersStore
	userID, err := usersStore.CreateUser(ctx, store.User{
		ProjectID:   projectID,
		AnonymousID: "anon_delete",
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
	controller, projectID := setupUsersController(t)
	ctx := context.Background()

	usersStore := controller.store.UsersStore
	userID, err := usersStore.CreateUser(ctx, store.User{
		ProjectID:   projectID,
		AnonymousID: "anon_version",
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
