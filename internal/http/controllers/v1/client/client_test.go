package v1

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/claim/rbac"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/consumer"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	teststore "github.com/lunogram/platform/internal/store/test"
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
	mgmt, usrs, _ := teststore.RunPostgreSQL(t)
	cfg := config.Node{
		Nats: config.Nats{
			URL: container.RunNATS(t),
		},
	}

	jet, err := pubsub.New(ctx, cfg)
	require.NoError(t, err)

	err = consumer.Bootstrap(ctx, logger, jet)
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet)
	usersState := subjects.NewState(usrs)

	controller := NewClientController(logger, usrs, usersState, pub)
	return &testClientController{
		ClientController: controller,
		mgmt:             management.NewState(mgmt),
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
				ProjectID:      projectID,
			}))
			w := httptest.NewRecorder()

			controller.PostEvents(w, req)

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
		ProjectID:      projectID,
	}))
	w := httptest.NewRecorder()

	controller.PostEvents(w, req)

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

	controller.PostEvents(w, req)

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
		// ProjectID is intentionally left as uuid.Nil
	}))
	w := httptest.NewRecorder()

	controller.PostEvents(w, req)

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
		ProjectID:      projectID,
	}))
	w := httptest.NewRecorder()

	controller.PostEvents(w, req)

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
		ProjectID:      projectID,
	}))
	w := httptest.NewRecorder()

	controller.PostEvents(w, req)

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
				ProjectID:      projectID,
			}))
			w := httptest.NewRecorder()

			controller.IdentifyUserClient(w, req)

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
				ProjectID:      projectID,
			}))
			w := httptest.NewRecorder()

			controller.IdentifyUserClient(w, req)

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

	controller.IdentifyUserClient(w, req)

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
		// ProjectID is intentionally left as uuid.Nil
	}))
	w := httptest.NewRecorder()

	controller.IdentifyUserClient(w, req)

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
		ProjectID:      projectID,
	}))
	w1 := httptest.NewRecorder()

	controller.IdentifyUserClient(w1, req1)

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
		ProjectID:      projectID,
	}))
	w2 := httptest.NewRecorder()

	controller.IdentifyUserClient(w2, req2)

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
		ProjectID:      projectID,
	}))
	w := httptest.NewRecorder()

	controller.IdentifyUserClient(w, req)

	assert.Equal(t, 200, w.Code)

	var response map[string]any
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.NotEmpty(t, response["id"])
}

func TestUpsertOrganizationClient(t *testing.T) {
	t.Parallel()

	type test struct {
		body       map[string]any
		statusCode int
	}

	tests := map[string]test{
		"create organization with all fields": {
			body: map[string]any{
				"external_id": "org_123",
				"name":        "Acme Corp",
				"data": map[string]any{
					"industry": "technology",
					"size":     "enterprise",
				},
			},
			statusCode: 200,
		},
		"create organization with minimal data": {
			body: map[string]any{
				"external_id": "org_456",
			},
			statusCode: 200,
		},
		"create organization with name only": {
			body: map[string]any{
				"external_id": "org_789",
				"name":        "Simple Corp",
			},
			statusCode: 200,
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

			body, err := json.Marshal(tc.body)
			require.NoError(t, err)

			req := httptest.NewRequest("POST", "/api/client/organizations", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(rbac.WithScope(req.Context(), &rbac.Scope{
				OrganizationID: orgID,
				ProjectID:      projectID,
			}))
			w := httptest.NewRecorder()

			controller.UpsertOrganizationClient(w, req)

			assert.Equal(t, tc.statusCode, w.Code)
			if w.Code == 200 {
				var response map[string]any
				err = json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.NotEmpty(t, response["id"])
				assert.Equal(t, tc.body["external_id"], response["external_id"])
			}
		})
	}
}

func TestUpsertOrganizationClientUpdate(t *testing.T) {
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

	// First upsert - create
	body1, err := json.Marshal(map[string]any{
		"external_id": "org_123",
		"name":        "Original Name",
		"data": map[string]any{
			"plan": "basic",
		},
	})
	require.NoError(t, err)

	req1 := httptest.NewRequest("POST", "/api/client/organizations", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	req1 = req1.WithContext(rbac.WithScope(req1.Context(), &rbac.Scope{
		OrganizationID: orgID,
		ProjectID:      projectID,
	}))
	w1 := httptest.NewRecorder()

	controller.UpsertOrganizationClient(w1, req1)
	assert.Equal(t, 200, w1.Code)

	var response1 map[string]any
	err = json.Unmarshal(w1.Body.Bytes(), &response1)
	require.NoError(t, err)
	orgInternalID := response1["id"]

	// Second upsert - update
	body2, err := json.Marshal(map[string]any{
		"external_id": "org_123",
		"name":        "Updated Name",
		"data": map[string]any{
			"plan": "enterprise",
		},
	})
	require.NoError(t, err)

	req2 := httptest.NewRequest("POST", "/api/client/organizations", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2 = req2.WithContext(rbac.WithScope(req2.Context(), &rbac.Scope{
		OrganizationID: orgID,
		ProjectID:      projectID,
	}))
	w2 := httptest.NewRecorder()

	controller.UpsertOrganizationClient(w2, req2)
	assert.Equal(t, 200, w2.Code)

	var response2 map[string]any
	err = json.Unmarshal(w2.Body.Bytes(), &response2)
	require.NoError(t, err)

	assert.Equal(t, orgInternalID, response2["id"])
	assert.Equal(t, "Updated Name", response2["name"])
}

func TestUpsertOrganizationClientMissingRBACScope(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)

	body, err := json.Marshal(map[string]any{
		"external_id": "org_123",
		"name":        "Test Org",
	})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/organizations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	controller.UpsertOrganizationClient(w, req)

	assert.Equal(t, 401, w.Code)
}

func TestUpsertOrganizationClientMissingProjectID(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)

	orgID, err := controller.mgmt.OrganizationsStore.CreateOrganization(t.Context(), "Test Org")
	require.NoError(t, err)

	body, err := json.Marshal(map[string]any{
		"external_id": "org_123",
		"name":        "Test Org",
	})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/organizations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithScope(req.Context(), &rbac.Scope{
		OrganizationID: orgID,
	}))
	w := httptest.NewRecorder()

	controller.UpsertOrganizationClient(w, req)

	assert.Equal(t, 401, w.Code)
}

func TestUpsertOrganizationClientInvalidRequest(t *testing.T) {
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

	req := httptest.NewRequest("POST", "/api/client/organizations", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithScope(req.Context(), &rbac.Scope{
		OrganizationID: orgID,
		ProjectID:      projectID,
	}))
	w := httptest.NewRecorder()

	controller.UpsertOrganizationClient(w, req)

	assert.Equal(t, 400, w.Code)
}

func TestAddOrganizationUserClient(t *testing.T) {
	t.Parallel()

	type test struct {
		body       map[string]any
		statusCode int
	}

	tests := map[string]test{
		"add user with data": {
			body: map[string]any{
				"organization_external_id": "org_123",
				"user_external_id":         "user_456",
				"data": map[string]any{
					"role":       "admin",
					"department": "engineering",
				},
			},
			statusCode: 200,
		},
		"add user without data": {
			body: map[string]any{
				"organization_external_id": "org_123",
				"user_external_id":         "user_789",
			},
			statusCode: 200,
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

			// Create the subject organization first
			orgBody, err := json.Marshal(map[string]any{
				"external_id": "org_123",
				"name":        "Test Subject Org",
			})
			require.NoError(t, err)

			orgReq := httptest.NewRequest("POST", "/api/client/organizations", bytes.NewReader(orgBody))
			orgReq.Header.Set("Content-Type", "application/json")
			orgReq = orgReq.WithContext(rbac.WithScope(orgReq.Context(), &rbac.Scope{
				OrganizationID: orgID,
				ProjectID:      projectID,
			}))
			orgW := httptest.NewRecorder()
			controller.UpsertOrganizationClient(orgW, orgReq)
			require.Equal(t, 200, orgW.Code)

			// Create the user
			userExternalID := tc.body["user_external_id"].(string)
			userBody, err := json.Marshal(map[string]any{
				"external_id": userExternalID,
				"email":       userExternalID + "@example.com",
			})
			require.NoError(t, err)

			userReq := httptest.NewRequest("POST", "/api/client/identify", bytes.NewReader(userBody))
			userReq.Header.Set("Content-Type", "application/json")
			userReq = userReq.WithContext(rbac.WithScope(userReq.Context(), &rbac.Scope{
				OrganizationID: orgID,
				ProjectID:      projectID,
			}))
			userW := httptest.NewRecorder()
			controller.IdentifyUserClient(userW, userReq)
			require.Equal(t, 200, userW.Code)

			// Add user to organization
			body, err := json.Marshal(tc.body)
			require.NoError(t, err)

			req := httptest.NewRequest("POST", "/api/client/organizations/users", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(rbac.WithScope(req.Context(), &rbac.Scope{
				OrganizationID: orgID,
				ProjectID:      projectID,
			}))
			w := httptest.NewRecorder()

			controller.AddOrganizationUserClient(w, req)

			assert.Equal(t, tc.statusCode, w.Code)
		})
	}
}

func TestAddOrganizationUserClientOrganizationNotFound(t *testing.T) {
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
		"organization_external_id": "nonexistent_org",
		"user_external_id":         "user_123",
	})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/organizations/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithScope(req.Context(), &rbac.Scope{
		OrganizationID: orgID,
		ProjectID:      projectID,
	}))
	w := httptest.NewRecorder()

	controller.AddOrganizationUserClient(w, req)

	assert.Equal(t, 404, w.Code)
}

func TestAddOrganizationUserClientUserNotFound(t *testing.T) {
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

	// Create the subject organization first
	orgBody, err := json.Marshal(map[string]any{
		"external_id": "org_123",
		"name":        "Test Subject Org",
	})
	require.NoError(t, err)

	orgReq := httptest.NewRequest("POST", "/api/client/organizations", bytes.NewReader(orgBody))
	orgReq.Header.Set("Content-Type", "application/json")
	orgReq = orgReq.WithContext(rbac.WithScope(orgReq.Context(), &rbac.Scope{
		OrganizationID: orgID,
		ProjectID:      projectID,
	}))
	orgW := httptest.NewRecorder()
	controller.UpsertOrganizationClient(orgW, orgReq)
	require.Equal(t, 200, orgW.Code)

	body, err := json.Marshal(map[string]any{
		"organization_external_id": "org_123",
		"user_external_id":         "nonexistent_user",
	})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/organizations/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithScope(req.Context(), &rbac.Scope{
		OrganizationID: orgID,
		ProjectID:      projectID,
	}))
	w := httptest.NewRecorder()

	controller.AddOrganizationUserClient(w, req)

	assert.Equal(t, 404, w.Code)
}

func TestAddOrganizationUserClientMissingRBACScope(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)

	body, err := json.Marshal(map[string]any{
		"organization_external_id": "org_123",
		"user_external_id":         "user_456",
	})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/organizations/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	controller.AddOrganizationUserClient(w, req)

	assert.Equal(t, 401, w.Code)
}

func TestRemoveOrganizationUserClient(t *testing.T) {
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

	// Create the subject organization
	orgBody, err := json.Marshal(map[string]any{
		"external_id": "org_123",
		"name":        "Test Subject Org",
	})
	require.NoError(t, err)

	orgReq := httptest.NewRequest("POST", "/api/client/organizations", bytes.NewReader(orgBody))
	orgReq.Header.Set("Content-Type", "application/json")
	orgReq = orgReq.WithContext(rbac.WithScope(orgReq.Context(), &rbac.Scope{
		OrganizationID: orgID,
		ProjectID:      projectID,
	}))
	orgW := httptest.NewRecorder()
	controller.UpsertOrganizationClient(orgW, orgReq)
	require.Equal(t, 200, orgW.Code)

	// Create the user
	userBody, err := json.Marshal(map[string]any{
		"external_id": "user_456",
		"email":       "user@example.com",
	})
	require.NoError(t, err)

	userReq := httptest.NewRequest("POST", "/api/client/identify", bytes.NewReader(userBody))
	userReq.Header.Set("Content-Type", "application/json")
	userReq = userReq.WithContext(rbac.WithScope(userReq.Context(), &rbac.Scope{
		OrganizationID: orgID,
		ProjectID:      projectID,
	}))
	userW := httptest.NewRecorder()
	controller.IdentifyUserClient(userW, userReq)
	require.Equal(t, 200, userW.Code)

	// Add user to organization
	addBody, err := json.Marshal(map[string]any{
		"organization_external_id": "org_123",
		"user_external_id":         "user_456",
	})
	require.NoError(t, err)

	addReq := httptest.NewRequest("POST", "/api/client/organizations/users", bytes.NewReader(addBody))
	addReq.Header.Set("Content-Type", "application/json")
	addReq = addReq.WithContext(rbac.WithScope(addReq.Context(), &rbac.Scope{
		OrganizationID: orgID,
		ProjectID:      projectID,
	}))
	addW := httptest.NewRecorder()
	controller.AddOrganizationUserClient(addW, addReq)
	require.Equal(t, 200, addW.Code)

	// Remove user from organization
	removeBody, err := json.Marshal(map[string]any{
		"organization_external_id": "org_123",
		"user_external_id":         "user_456",
	})
	require.NoError(t, err)

	removeReq := httptest.NewRequest("DELETE", "/api/client/organizations/users", bytes.NewReader(removeBody))
	removeReq.Header.Set("Content-Type", "application/json")
	removeReq = removeReq.WithContext(rbac.WithScope(removeReq.Context(), &rbac.Scope{
		OrganizationID: orgID,
		ProjectID:      projectID,
	}))
	removeW := httptest.NewRecorder()

	controller.RemoveOrganizationUserClient(removeW, removeReq)

	assert.Equal(t, 204, removeW.Code)
}

func TestRemoveOrganizationUserClientOrganizationNotFound(t *testing.T) {
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
		"organization_external_id": "nonexistent_org",
		"user_external_id":         "user_123",
	})
	require.NoError(t, err)

	req := httptest.NewRequest("DELETE", "/api/client/organizations/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithScope(req.Context(), &rbac.Scope{
		OrganizationID: orgID,
		ProjectID:      projectID,
	}))
	w := httptest.NewRecorder()

	controller.RemoveOrganizationUserClient(w, req)

	assert.Equal(t, 404, w.Code)
}

func TestRemoveOrganizationUserClientUserNotFound(t *testing.T) {
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

	// Create the subject organization
	orgBody, err := json.Marshal(map[string]any{
		"external_id": "org_123",
		"name":        "Test Subject Org",
	})
	require.NoError(t, err)

	orgReq := httptest.NewRequest("POST", "/api/client/organizations", bytes.NewReader(orgBody))
	orgReq.Header.Set("Content-Type", "application/json")
	orgReq = orgReq.WithContext(rbac.WithScope(orgReq.Context(), &rbac.Scope{
		OrganizationID: orgID,
		ProjectID:      projectID,
	}))
	orgW := httptest.NewRecorder()
	controller.UpsertOrganizationClient(orgW, orgReq)
	require.Equal(t, 200, orgW.Code)

	body, err := json.Marshal(map[string]any{
		"organization_external_id": "org_123",
		"user_external_id":         "nonexistent_user",
	})
	require.NoError(t, err)

	req := httptest.NewRequest("DELETE", "/api/client/organizations/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithScope(req.Context(), &rbac.Scope{
		OrganizationID: orgID,
		ProjectID:      projectID,
	}))
	w := httptest.NewRecorder()

	controller.RemoveOrganizationUserClient(w, req)

	assert.Equal(t, 404, w.Code)
}

func TestRemoveOrganizationUserClientMissingRBACScope(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)

	body, err := json.Marshal(map[string]any{
		"organization_external_id": "org_123",
		"user_external_id":         "user_456",
	})
	require.NoError(t, err)

	req := httptest.NewRequest("DELETE", "/api/client/organizations/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	controller.RemoveOrganizationUserClient(w, req)

	assert.Equal(t, 401, w.Code)
}

func TestPostOrganizationEventsClient(t *testing.T) {
	t.Parallel()

	type test struct {
		events     []map[string]any
		statusCode int
	}

	tests := map[string]test{
		"single event with data": {
			events: []map[string]any{
				{
					"organization_external_id": "org_123",
					"name":                     "subscription_upgraded",
					"data": map[string]any{
						"plan":  "enterprise",
						"seats": 100,
					},
				},
			},
			statusCode: 202,
		},
		"single event without data": {
			events: []map[string]any{
				{
					"organization_external_id": "org_123",
					"name":                     "account_activated",
				},
			},
			statusCode: 202,
		},
		"multiple events": {
			events: []map[string]any{
				{
					"organization_external_id": "org_123",
					"name":                     "feature_enabled",
					"data": map[string]any{
						"feature": "advanced_analytics",
					},
				},
				{
					"organization_external_id": "org_123",
					"name":                     "user_invited",
					"data": map[string]any{
						"invitee_email": "new@example.com",
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

			orgID, err := controller.mgmt.OrganizationsStore.CreateOrganization(t.Context(), "Test Org")
			require.NoError(t, err)

			projectID, err := controller.mgmt.ProjectsStore.CreateProject(t.Context(), management.Project{
				OrganizationID: &orgID,
				Name:           "Test Project",
				Timezone:       "UTC",
				Locale:         "en",
			})
			require.NoError(t, err)

			// Create the subject organization first
			orgBody, err := json.Marshal(map[string]any{
				"external_id": "org_123",
				"name":        "Test Subject Org",
			})
			require.NoError(t, err)

			orgReq := httptest.NewRequest("POST", "/api/client/organizations", bytes.NewReader(orgBody))
			orgReq.Header.Set("Content-Type", "application/json")
			orgReq = orgReq.WithContext(rbac.WithScope(orgReq.Context(), &rbac.Scope{
				OrganizationID: orgID,
				ProjectID:      projectID,
			}))
			orgW := httptest.NewRecorder()
			controller.UpsertOrganizationClient(orgW, orgReq)
			require.Equal(t, 200, orgW.Code)

			// Post organization events
			body, err := json.Marshal(tc.events)
			require.NoError(t, err)

			req := httptest.NewRequest("POST", "/api/client/organizations/events", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(rbac.WithScope(req.Context(), &rbac.Scope{
				OrganizationID: orgID,
				ProjectID:      projectID,
			}))
			w := httptest.NewRecorder()

			controller.PostOrganizationEventsClient(w, req)

			assert.Equal(t, tc.statusCode, w.Code)
		})
	}
}

func TestPostOrganizationEventsClientNonexistentOrg(t *testing.T) {
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
			"organization_external_id": "nonexistent_org",
			"name":                     "test_event",
		},
	}

	body, err := json.Marshal(events)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/organizations/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithScope(req.Context(), &rbac.Scope{
		OrganizationID: orgID,
		ProjectID:      projectID,
	}))
	w := httptest.NewRecorder()

	controller.PostOrganizationEventsClient(w, req)

	// Events for nonexistent orgs are skipped, not rejected
	assert.Equal(t, 202, w.Code)
}

func TestPostOrganizationEventsClientMissingRBACScope(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)

	events := []map[string]any{
		{
			"organization_external_id": "org_123",
			"name":                     "test_event",
		},
	}

	body, err := json.Marshal(events)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/organizations/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	controller.PostOrganizationEventsClient(w, req)

	assert.Equal(t, 401, w.Code)
}

func TestPostOrganizationEventsClientMissingProjectID(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)

	orgID, err := controller.mgmt.OrganizationsStore.CreateOrganization(t.Context(), "Test Org")
	require.NoError(t, err)

	events := []map[string]any{
		{
			"organization_external_id": "org_123",
			"name":                     "test_event",
		},
	}

	body, err := json.Marshal(events)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/organizations/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithScope(req.Context(), &rbac.Scope{
		OrganizationID: orgID,
	}))
	w := httptest.NewRecorder()

	controller.PostOrganizationEventsClient(w, req)

	assert.Equal(t, 401, w.Code)
}

func TestPostOrganizationEventsClientInvalidRequest(t *testing.T) {
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

	req := httptest.NewRequest("POST", "/api/client/organizations/events", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithScope(req.Context(), &rbac.Scope{
		OrganizationID: orgID,
		ProjectID:      projectID,
	}))
	w := httptest.NewRecorder()

	controller.PostOrganizationEventsClient(w, req)

	assert.Equal(t, 400, w.Code)
}

func TestPostOrganizationEventsClientWithNestedData(t *testing.T) {
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

	// Create the subject organization first
	orgBody, err := json.Marshal(map[string]any{
		"external_id": "org_123",
		"name":        "Test Subject Org",
	})
	require.NoError(t, err)

	orgReq := httptest.NewRequest("POST", "/api/client/organizations", bytes.NewReader(orgBody))
	orgReq.Header.Set("Content-Type", "application/json")
	orgReq = orgReq.WithContext(rbac.WithScope(orgReq.Context(), &rbac.Scope{
		OrganizationID: orgID,
		ProjectID:      projectID,
	}))
	orgW := httptest.NewRecorder()
	controller.UpsertOrganizationClient(orgW, orgReq)
	require.Equal(t, 200, orgW.Code)

	events := []map[string]any{
		{
			"organization_external_id": "org_123",
			"name":                     "complex_event",
			"data": map[string]any{
				"subscription": map[string]any{
					"plan":     "enterprise",
					"features": []string{"analytics", "api", "support"},
					"billing": map[string]any{
						"amount":   999.99,
						"currency": "USD",
						"interval": "monthly",
					},
				},
				"users_count": 50,
			},
		},
	}

	body, err := json.Marshal(events)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/organizations/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithScope(req.Context(), &rbac.Scope{
		OrganizationID: orgID,
		ProjectID:      projectID,
	}))
	w := httptest.NewRecorder()

	controller.PostOrganizationEventsClient(w, req)

	assert.Equal(t, 202, w.Code)
}
