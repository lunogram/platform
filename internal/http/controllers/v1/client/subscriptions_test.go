package v1

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/http/controllers/v1/client/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/users"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func setupSubscriptionsController(t *testing.T) (*SubscriptionsController, uuid.UUID, uuid.UUID) {
	t.Helper()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	postgresURI := container.RunPostgreSQL(t)

	err := management.Migrate(postgresURI)
	require.NoError(t, err)

	err = users.Migrate(postgresURI)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, store.Config{
		ManagementURI: postgresURI,
		UsersURI:      postgresURI,
		JourneyURI:    postgresURI,
	})
	require.NoError(t, err)

	mgmt := management.NewState(db.Management)
	usrs := users.NewState(db.Users)

	// Create project
	projectsStore := management.NewProjectsStore(db.Management)
	projectID, err := projectsStore.CreateProject(ctx, management.Project{
		Name:     "Test Project",
		Timezone: "UTC",
		Locale:   "en-US",
	})
	require.NoError(t, err)

	// Create user
	email := "test@example.com"
	userID, err := usrs.CreateUser(ctx, users.User{
		ProjectID: projectID,
		Email:     &email,
		Data:      json.RawMessage("{}"),
	})
	require.NoError(t, err)

	controller, err := NewSubscriptionsController(logger, db.Management, mgmt, usrs)
	require.NoError(t, err)

	return controller, projectID, userID
}

func TestGetPreferencesPage(t *testing.T) {
	t.Parallel()

	controller, projectID, userID := setupSubscriptionsController(t)

	type test struct {
		projectID uuid.UUID
		userID    uuid.UUID
		code      int
	}

	tests := map[string]test{
		"success": {
			projectID: projectID,
			userID:    userID,
			code:      200,
		},
		"user not found": {
			projectID: projectID,
			userID:    uuid.New(),
			code:      404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/preferences/"+test.projectID.String()+"/"+test.userID.String(), nil)
			controller.GetPreferencesPage(res, req, test.projectID, test.userID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 200 {
				require.Contains(t, res.Body.String(), "html")
			}
		})
	}
}

func TestUpdatePreferences(t *testing.T) {
	t.Parallel()

	controller, projectID, userID := setupSubscriptionsController(t)

	type test struct {
		projectID uuid.UUID
		userID    uuid.UUID
		formData  url.Values
		code      int
	}

	tests := map[string]test{
		"success": {
			projectID: projectID,
			userID:    userID,
			formData:  url.Values{},
			code:      303,
		},
		"user not found": {
			projectID: projectID,
			userID:    uuid.New(),
			formData:  url.Values{},
			code:      404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/preferences/"+test.projectID.String()+"/"+test.userID.String(), strings.NewReader(test.formData.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			controller.UpdatePreferences(res, req, test.projectID, test.userID)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

func TestEmailUnsubscribe(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	postgresURI := container.RunPostgreSQL(t)

	err := management.Migrate(postgresURI)
	require.NoError(t, err)

	err = users.Migrate(postgresURI)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, store.Config{
		ManagementURI: postgresURI,
		UsersURI:      postgresURI,
		JourneyURI:    postgresURI,
	})
	require.NoError(t, err)

	mgmt := management.NewState(db.Management)
	usrs := users.NewState(db.Users)

	// Create project
	projectsStore := management.NewProjectsStore(db.Management)
	projectID, err := projectsStore.CreateProject(ctx, management.Project{
		Name:     "Test Project",
		Timezone: "UTC",
		Locale:   "en-US",
	})
	require.NoError(t, err)

	// Create user
	email := "test@example.com"
	userID, err := usrs.CreateUser(ctx, users.User{
		ProjectID: projectID,
		Email:     &email,
		Data:      json.RawMessage("{}"),
	})
	require.NoError(t, err)

	// Create subscription
	subscriptionsStore := management.NewSubscriptionsStore(db.Management)
	subscriptionID, err := subscriptionsStore.CreateSubscription(ctx, management.Subscription{
		ProjectID: projectID,
		Name:      "Test Subscription",
		Channel:   "email",
	})
	require.NoError(t, err)

	// Create campaign with subscription
	campaignsStore := management.NewCampaignsStore(db.Management)
	campaignID, err := campaignsStore.CreateCampaign(ctx, management.Campaign{
		ProjectID:      projectID,
		Name:           "Test Campaign",
		Channel:        "email",
		SubscriptionID: &subscriptionID,
	})
	require.NoError(t, err)

	controller, err := NewSubscriptionsController(logger, db.Management, mgmt, usrs)
	require.NoError(t, err)

	type test struct {
		link string
		code int
	}

	tests := map[string]test{
		"success": {
			link: "?u=" + userID.String() + "&c=" + campaignID.String(),
			code: 200,
		},
		"invalid link": {
			link: "invalid",
			code: 400,
		},
		"invalid user id": {
			link: "?u=invalid&c=" + campaignID.String(),
			code: 400,
		},
		"invalid campaign id": {
			link: "?u=" + userID.String() + "&c=invalid",
			code: 400,
		},
		"campaign not found": {
			link: "?u=" + userID.String() + "&c=" + uuid.New().String(),
			code: 404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/unsubscribe?link="+url.QueryEscape(test.link), nil)
			params := oapi.EmailUnsubscribeParams{
				Link: test.link,
			}
			controller.EmailUnsubscribe(res, req, params)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 200 {
				require.Contains(t, res.Body.String(), "html")
			}
		})
	}
}
