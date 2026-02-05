package v1

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/claim/rbac"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/consumer"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

type testClientController struct {
	*ClientController
	mgmt *management.State
}

func setupClientController(t *testing.T) *testClientController {
	t.Helper()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	uri := container.RunPostgreSQL(t)
	cfg := config.Node{
		Nats: config.Nats{
			URL: container.RunNATS(t),
		},
	}
	mgmtConfig := management.Config{
		URI: uri,
	}

	err := management.Migrate(mgmtConfig)
	require.NoError(t, err)

	db, err := management.New(ctx, logger, mgmtConfig)
	require.NoError(t, err)

	jet, err := pubsub.New(ctx, cfg)
	require.NoError(t, err)

	err = consumer.Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)
	usersState := users.NewState(db)

	controller := NewClientController(logger, db, usersState, pub)
	return &testClientController{
		ClientController: controller,
		mgmt:             management.NewState(db),
	}
}

func TestPostEvents(t *testing.T) {
	t.Parallel()

	type test struct {
		events     []map[string]any
		statusCode int
	}

	tests := map[string]test{
		"single event with external_id": {
			events: []map[string]any{
				{
					"name":        "purchase_completed",
					"external_id": "user_123",
					"data": map[string]any{
						"amount":     99.99,
						"product_id": "prod_456",
					},
				},
			},
			statusCode: 202,
		},
		"single event with anonymous_id": {
			events: []map[string]any{
				{
					"name":         "page_viewed",
					"anonymous_id": "anon_abc",
					"data": map[string]any{
						"page": "/home",
					},
				},
			},
			statusCode: 202,
		},
		"multiple events": {
			events: []map[string]any{
				{
					"name":        "cart_updated",
					"external_id": "user_123",
				},
				{
					"name":         "product_viewed",
					"anonymous_id": "anon_xyz",
				},
			},
			statusCode: 202,
		},
		"event with user data": {
			events: []map[string]any{
				{
					"name":        "signup",
					"external_id": "user_789",
					"user": map[string]any{
						"email":    "user@example.com",
						"timezone": "America/New_York",
						"locale":   "en-US",
						"data": map[string]any{
							"plan": "premium",
						},
					},
				},
			},
			statusCode: 202,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			controller := setupClientController(t)

			// Create organization and project for the test
			orgID, err := controller.mgmt.OrganizationsStore.CreateOrganization(t.Context(), "Test Org")
			require.NoError(t, err)

			projectID, err := controller.mgmt.ProjectsStore.CreateProject(t.Context(), management.Project{
				OrganizationID: &orgID,
				Name:           "Test Project",
				Timezone:       "UTC",
				Locale:         "en",
			})
			require.NoError(t, err)

			body, err := json.Marshal(tc.events)
			require.NoError(t, err)

			req := httptest.NewRequest("POST", "/api/client/events", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(rbac.WithScope(req.Context(), &rbac.Scope{
				OrganizationID: orgID,
			}))
			w := httptest.NewRecorder()

			controller.PostEvents(w, req, projectID)

			assert.Equal(t, tc.statusCode, w.Code)
		})
	}
}

func TestPostEventsInvalidRequest(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)

	orgID, err := controller.mgmt.OrganizationsStore.CreateOrganization(t.Context(), "Test Org")
	require.NoError(t, err)

	projectID, err := controller.mgmt.ProjectsStore.CreateProject(t.Context(), management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/events", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithScope(req.Context(), &rbac.Scope{
		OrganizationID: orgID,
	}))
	w := httptest.NewRecorder()

	controller.PostEvents(w, req, projectID)

	assert.Equal(t, 400, w.Code)
}

func TestPostEventsMissingRBACScope(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)

	events := []map[string]any{
		{
			"name":        "test_event",
			"external_id": "user_123",
		},
	}

	body, err := json.Marshal(events)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	controller.PostEvents(w, req, uuid.Nil)

	assert.Equal(t, 401, w.Code)
}

func TestPostEventsMissingProjectID(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)

	orgID, err := controller.mgmt.OrganizationsStore.CreateOrganization(t.Context(), "Test Org")
	require.NoError(t, err)

	events := []map[string]any{
		{
			"name":        "test_event",
			"external_id": "user_123",
		},
	}

	body, err := json.Marshal(events)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithScope(req.Context(), &rbac.Scope{
		OrganizationID: orgID,
	}))
	w := httptest.NewRecorder()

	controller.PostEvents(w, req, uuid.Nil)

	assert.Equal(t, 401, w.Code)
}

func TestPostEventsWithNestedData(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)

	orgID, err := controller.mgmt.OrganizationsStore.CreateOrganization(t.Context(), "Test Org")
	require.NoError(t, err)

	projectID, err := controller.mgmt.ProjectsStore.CreateProject(t.Context(), management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	events := []map[string]any{
		{
			"name":        "complex_event",
			"external_id": "user_123",
			"data": map[string]any{
				"product": map[string]any{
					"id":    "prod_123",
					"name":  "Widget",
					"price": 99.99,
					"metadata": map[string]any{
						"sku":      "WDG-001",
						"category": "electronics",
					},
				},
				"quantity": 2,
				"tags":     []string{"sale", "featured"},
			},
		},
	}

	body, err := json.Marshal(events)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithScope(req.Context(), &rbac.Scope{
		OrganizationID: orgID,
	}))
	w := httptest.NewRecorder()

	controller.PostEvents(w, req, projectID)

	assert.Equal(t, 202, w.Code)
}

func TestPostEventsEmptyArray(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)

	orgID, err := controller.mgmt.OrganizationsStore.CreateOrganization(t.Context(), "Test Org")
	require.NoError(t, err)

	projectID, err := controller.mgmt.ProjectsStore.CreateProject(t.Context(), management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	events := []map[string]any{}

	body, err := json.Marshal(events)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithScope(req.Context(), &rbac.Scope{
		OrganizationID: orgID,
	}))
	w := httptest.NewRecorder()

	controller.PostEvents(w, req, projectID)

	assert.Equal(t, 202, w.Code)
}

func TestClientIdentifyUser(t *testing.T) {
	t.Parallel()

	type test struct {
		body       map[string]any
		statusCode int
	}

	tests := map[string]test{
		"identify with external_id": {
			body: map[string]any{
				"external_id": "user_123",
				"email":       "user@example.com",
				"timezone":    "America/Chicago",
				"locale":      "en",
				"data": map[string]any{
					"first_name": "John",
					"last_name":  "Smith",
				},
			},
			statusCode: 200,
		},
		"identify with anonymous_id": {
			body: map[string]any{
				"anonymous_id": "anon_abc",
				"email":        "test@test.com",
			},
			statusCode: 200,
		},
		"identify with minimal data": {
			body: map[string]any{
				"external_id": "user_456",
			},
			statusCode: 200,
		},
		"identify with phone": {
			body: map[string]any{
				"external_id": "user_789",
				"phone":       "+1234567890",
				"timezone":    "Europe/Amsterdam",
			},
			statusCode: 200,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			controller := setupClientController(t)

			// Create organization and project for the test
			orgID, err := controller.mgmt.OrganizationsStore.CreateOrganization(t.Context(), "Test Org")
			require.NoError(t, err)

			projectID, err := controller.mgmt.ProjectsStore.CreateProject(t.Context(), management.Project{
				OrganizationID: &orgID,
				Name:           "Test Project",
				Timezone:       "UTC",
				Locale:         "en",
			})
			require.NoError(t, err)

			body, err := json.Marshal(tc.body)
			require.NoError(t, err)

			req := httptest.NewRequest("POST", "/api/client/identify", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(rbac.WithScope(req.Context(), &rbac.Scope{
				OrganizationID: orgID,
			}))
			w := httptest.NewRecorder()

			controller.IdentifyUser(w, req, projectID)

			assert.Equal(t, tc.statusCode, w.Code)
			if w.Code == 200 {
				var response map[string]any
				err = json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.NotEmpty(t, response["id"])
			}
		})
	}
}

func TestClientIdentifyUserInvalidRequest(t *testing.T) {
	t.Parallel()

	type test struct {
		body       map[string]any
		statusCode int
	}

	tests := map[string]test{
		"missing both identifiers": {
			body: map[string]any{
				"email": "test@test.com",
			},
			statusCode: 400,
		},
		"invalid json": {
			body:       nil,
			statusCode: 400,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			controller := setupClientController(t)

			orgID, err := controller.mgmt.OrganizationsStore.CreateOrganization(t.Context(), "Test Org")
			require.NoError(t, err)

			projectID, err := controller.mgmt.ProjectsStore.CreateProject(t.Context(), management.Project{
				OrganizationID: &orgID,
				Name:           "Test Project",
				Timezone:       "UTC",
				Locale:         "en",
			})
			require.NoError(t, err)

			var body []byte
			if tc.body != nil {
				body, err = json.Marshal(tc.body)
				require.NoError(t, err)
			} else {
				body = []byte("invalid")
			}

			req := httptest.NewRequest("POST", "/api/client/identify", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(rbac.WithScope(req.Context(), &rbac.Scope{
				OrganizationID: orgID,
			}))
			w := httptest.NewRecorder()

			controller.IdentifyUser(w, req, projectID)

			assert.Equal(t, tc.statusCode, w.Code)
		})
	}
}

func TestClientIdentifyUserMissingRBACScope(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)

	body, err := json.Marshal(map[string]any{
		"external_id": "user_123",
		"email":       "test@test.com",
	})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/identify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	controller.IdentifyUser(w, req, uuid.Nil)

	assert.Equal(t, 401, w.Code)
}

func TestClientIdentifyUserMissingProjectID(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)

	orgID, err := controller.mgmt.OrganizationsStore.CreateOrganization(t.Context(), "Test Org")
	require.NoError(t, err)

	body, err := json.Marshal(map[string]any{
		"external_id": "user_123",
		"email":       "test@test.com",
	})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/identify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithScope(req.Context(), &rbac.Scope{
		OrganizationID: orgID,
	}))
	w := httptest.NewRecorder()

	controller.IdentifyUser(w, req, uuid.Nil)

	assert.Equal(t, 401, w.Code)
}

func TestClientIdentifyUserUpdateExisting(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)

	orgID, err := controller.mgmt.OrganizationsStore.CreateOrganization(t.Context(), "Test Org")
	require.NoError(t, err)

	projectID, err := controller.mgmt.ProjectsStore.CreateProject(t.Context(), management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	// First identify call
	body1, err := json.Marshal(map[string]any{
		"external_id": "user_123",
		"email":       "original@example.com",
		"timezone":    "America/New_York",
		"data": map[string]any{
			"first_name": "John",
		},
	})
	require.NoError(t, err)

	req1 := httptest.NewRequest("POST", "/api/client/identify", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	req1 = req1.WithContext(rbac.WithScope(req1.Context(), &rbac.Scope{
		OrganizationID: orgID,
	}))
	w1 := httptest.NewRecorder()

	controller.IdentifyUser(w1, req1, projectID)

	assert.Equal(t, 200, w1.Code)

	var response1 map[string]any
	err = json.Unmarshal(w1.Body.Bytes(), &response1)
	require.NoError(t, err)
	userID1 := response1["id"]

	// Second identify call with updated data
	body2, err := json.Marshal(map[string]any{
		"external_id": "user_123",
		"email":       "updated@example.com",
		"timezone":    "Europe/Amsterdam",
		"data": map[string]any{
			"first_name": "John",
			"last_name":  "Doe",
		},
	})
	require.NoError(t, err)

	req2 := httptest.NewRequest("POST", "/api/client/identify", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2 = req2.WithContext(rbac.WithScope(req2.Context(), &rbac.Scope{
		OrganizationID: orgID,
	}))
	w2 := httptest.NewRecorder()

	controller.IdentifyUser(w2, req2, projectID)

	assert.Equal(t, 200, w2.Code)

	var response2 map[string]any
	err = json.Unmarshal(w2.Body.Bytes(), &response2)
	require.NoError(t, err)

	// Should be the same user ID
	assert.Equal(t, userID1, response2["id"])

	// Email should be updated
	if email, ok := response2["email"].(string); ok {
		assert.Equal(t, "updated@example.com", email)
	}
}

func TestClientIdentifyUserWithBothIdentifiers(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)

	orgID, err := controller.mgmt.OrganizationsStore.CreateOrganization(t.Context(), "Test Org")
	require.NoError(t, err)

	projectID, err := controller.mgmt.ProjectsStore.CreateProject(t.Context(), management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	body, err := json.Marshal(map[string]any{
		"external_id":  "user_123",
		"anonymous_id": "anon_abc",
		"email":        "test@test.com",
	})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/identify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithScope(req.Context(), &rbac.Scope{
		OrganizationID: orgID,
	}))
	w := httptest.NewRecorder()

	controller.IdentifyUser(w, req, projectID)

	assert.Equal(t, 200, w.Code)

	var response map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotEmpty(t, response["id"])
}
