package v1

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/pkg/container"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestUnsubscribeEmail(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	cfg := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(cfg.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, cfg.Store)
	require.NoError(t, err)
	defer db.Close()

	stores := store.NewStores(db)
	controller := NewSubscriptionsController(logger, db)

	// Setup test data
	projectID := uuid.New()
	_, err = db.Exec(`INSERT INTO projects (id, name) VALUES ($1, $2)`, projectID, "Test Project")
	require.NoError(t, err)

	userID := uuid.New()
	_, err = db.Exec(`INSERT INTO users (id, project_id, anonymous_id) VALUES ($1, $2, $3)`,
		userID, projectID, "anon_"+userID.String())
	require.NoError(t, err)

	subscriptionID, err := stores.CreateSubscription(ctx, store.Subscription{
		ProjectID: projectID,
		Name:      "Test Newsletter",
		Channel:   "email",
		IsPublic:  true,
	})
	require.NoError(t, err)

	campaignID := uuid.New()
	_, err = db.Exec(`INSERT INTO campaigns (id, project_id, name, channel, subscription_id) VALUES ($1, $2, $3, $4, $5)`,
		campaignID, projectID, "Test Campaign", "email", subscriptionID)
	require.NoError(t, err)

	type test struct {
		userID         string
		campaignID     string
		expectedStatus int
		expectedBody   string
	}

	tests := map[string]test{
		"successful unsubscribe": {
			userID:         userID.String(),
			campaignID:     campaignID.String(),
			expectedStatus: http.StatusOK,
			expectedBody:   "You have been unsubscribed",
		},
		"missing user_id": {
			campaignID:     campaignID.String(),
			expectedStatus: http.StatusBadRequest,
		},
		"missing campaign_id": {
			userID:         userID.String(),
			expectedStatus: http.StatusBadRequest,
		},
		"invalid user_id": {
			userID:         "invalid",
			campaignID:     campaignID.String(),
			expectedStatus: http.StatusBadRequest,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			queryString := url.Values{}
			if tc.userID != "" {
				queryString.Set("user_id", tc.userID)
			}
			if tc.campaignID != "" {
				queryString.Set("campaign_id", tc.campaignID)
			}

			req := httptest.NewRequest(http.MethodGet, "/unsubscribe/email?"+queryString.Encode(), nil)
			w := httptest.NewRecorder()

			controller.UnsubscribeEmail(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)
			if tc.expectedBody != "" {
				assert.Contains(t, w.Body.String(), tc.expectedBody)
			}
		})
	}
}

func TestGetPreferences(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	cfg := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(cfg.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, cfg.Store)
	require.NoError(t, err)
	defer db.Close()

	stores := store.NewStores(db)
	controller := NewSubscriptionsController(logger, db)

	// Setup test data
	projectID := uuid.New()
	_, err = db.Exec(`INSERT INTO projects (id, name) VALUES ($1, $2)`, projectID, "Test Project")
	require.NoError(t, err)

	userID := uuid.New()
	_, err = db.Exec(`INSERT INTO users (id, project_id, anonymous_id) VALUES ($1, $2, $3)`,
		userID, projectID, "anon_"+userID.String())
	require.NoError(t, err)

	_, err = stores.CreateSubscription(ctx, store.Subscription{
		ProjectID: projectID,
		Name:      "Marketing Newsletter",
		Channel:   "email",
		IsPublic:  true,
	})
	require.NoError(t, err)

	type test struct {
		userIDParam    string
		projectIDParam string
		expectedStatus int
		expectedBody   string
	}

	tests := map[string]test{
		"successful render": {
			userIDParam:    userID.String(),
			projectIDParam: projectID.String(),
			expectedStatus: http.StatusOK,
			expectedBody:   "Communication Preferences",
		},
		"missing project_id": {
			userIDParam:    userID.String(),
			expectedStatus: http.StatusBadRequest,
		},
		"invalid user_id": {
			userIDParam:    "invalid",
			projectIDParam: projectID.String(),
			expectedStatus: http.StatusBadRequest,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			queryString := url.Values{}
			if tc.projectIDParam != "" {
				queryString.Set("project_id", tc.projectIDParam)
			}

			req := httptest.NewRequest(http.MethodGet, "/preferences/"+tc.userIDParam+"?"+queryString.Encode(), nil)
			w := httptest.NewRecorder()

			controller.GetPreferences(w, req, tc.userIDParam)

			assert.Equal(t, tc.expectedStatus, w.Code)
			if tc.expectedBody != "" {
				assert.Contains(t, w.Body.String(), tc.expectedBody)
			}
		})
	}
}

func TestUpdatePreferences(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	cfg := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(cfg.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, cfg.Store)
	require.NoError(t, err)
	defer db.Close()

	stores := store.NewStores(db)
	controller := NewSubscriptionsController(logger, db)

	// Setup test data
	projectID := uuid.New()
	_, err = db.Exec(`INSERT INTO projects (id, name) VALUES ($1, $2)`, projectID, "Test Project")
	require.NoError(t, err)

	userID := uuid.New()
	_, err = db.Exec(`INSERT INTO users (id, project_id, anonymous_id) VALUES ($1, $2, $3)`,
		userID, projectID, "anon_"+userID.String())
	require.NoError(t, err)

	sub1ID, err := stores.CreateSubscription(ctx, store.Subscription{
		ProjectID: projectID,
		Name:      "Newsletter",
		Channel:   "email",
		IsPublic:  true,
	})
	require.NoError(t, err)

	sub2ID, err := stores.CreateSubscription(ctx, store.Subscription{
		ProjectID: projectID,
		Name:      "Updates",
		Channel:   "sms",
		IsPublic:  true,
	})
	require.NoError(t, err)

	type test struct {
		selectedIDs    []string
		expectedStatus int
	}

	tests := map[string]test{
		"select one subscription": {
			selectedIDs:    []string{sub1ID.String()},
			expectedStatus: http.StatusSeeOther,
		},
		"select multiple subscriptions": {
			selectedIDs:    []string{sub1ID.String(), sub2ID.String()},
			expectedStatus: http.StatusSeeOther,
		},
		"select none": {
			selectedIDs:    []string{},
			expectedStatus: http.StatusSeeOther,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			form := url.Values{}
			for _, id := range tc.selectedIDs {
				form.Add("subscription_ids", id)
			}

			req := httptest.NewRequest(http.MethodPost, "/preferences/"+userID.String()+"?project_id="+projectID.String(), strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			w := httptest.NewRecorder()

			controller.UpdatePreferences(w, req, userID.String())

			assert.Equal(t, tc.expectedStatus, w.Code)

			// Verify the subscriptions were updated correctly
			subscriptions, _, err := stores.GetUserSubscriptions(ctx, projectID, userID, store.Pagination{Limit: 10, Offset: 0})
			require.NoError(t, err)

			selectedMap := make(map[string]bool)
			for _, id := range tc.selectedIDs {
				selectedMap[id] = true
			}

			for _, sub := range subscriptions {
				if selectedMap[sub.SubscriptionID.String()] {
					assert.Equal(t, "subscribed", sub.State, "subscription %s should be subscribed", sub.Name)
				} else {
					assert.Equal(t, "unsubscribed", sub.State, "subscription %s should be unsubscribed", sub.Name)
				}
			}
		})
	}
}
