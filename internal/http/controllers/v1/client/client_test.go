package v1

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/consumer"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

type testClientController struct {
	*ClientController
	mgmt *management.State
}

// actorContext creates an RBAC actor with the "client" project role and writes
// the necessary relationship tuples so that permission checks succeed. It
// returns the enriched request context that carries the actor.
//
// When projectID is uuid.Nil the actor is created without a project (useful for
// "missing project" tests) and no tuples are written.
func (tc *testClientController) actorContext(t *testing.T, orgID, projectID uuid.UUID) *rbac.Actor {
	t.Helper()

	opts := []rbac.ActorOption{rbac.WithOrganizationID(orgID)}
	if projectID != uuid.Nil {
		opts = append(opts, rbac.WithProjectID(projectID))
	}

	actor := rbac.NewActor(rbac.ActorAPIKey, uuid.New().String(), opts...)

	engine, _ := rbac.TestSetup(t, t.Context(), actor, "member", "client")
	tc.ClientController.engine = engine

	return actor
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

	err = consumer.Bootstrap(ctx, logger, jet, "")
	require.NoError(t, err)

	pub := pubsub.NewPublisher(jet, "")
	usersState := subjects.NewState(usrs, zap.NewNop())

	// Start with a bare engine; tests that need permissions call actorContext.
	controller := NewClientController(logger, usrs, usersState, pub, rbac.NewTestEngine(t))
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
					"name":       "purchase_completed",
					"identifier": []map[string]any{{"source": "default", "external_id": "user_123"}},
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
					"name":       "page_viewed",
					"identifier": []map[string]any{{"source": "anonymous", "external_id": "anon_abc"}},
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
					"name":       "cart_updated",
					"identifier": []map[string]any{{"source": "default", "external_id": "user_123"}},
				},
				{
					"name":       "product_viewed",
					"identifier": []map[string]any{{"source": "anonymous", "external_id": "anon_xyz"}},
				},
			},
			statusCode: 202,
		},
		"event with user identifiers": {
			events: []map[string]any{
				{
					"name":       "signup",
					"identifier": []map[string]any{{"source": "default", "external_id": "user_789"}},
					"data": map[string]any{
						"plan": "premium",
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

			body, err := json.Marshal(tc.events)
			require.NoError(t, err)

			req := httptest.NewRequest("POST", "/api/client/users/events", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			actor := controller.actorContext(t, orgID, projectID)
			req = req.WithContext(rbac.WithActor(req.Context(), actor))
			w := httptest.NewRecorder()

			controller.PostUserEvents(w, req)

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

	req := httptest.NewRequest("POST", "/api/client/users/events", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	actor := controller.actorContext(t, orgID, projectID)
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
	w := httptest.NewRecorder()

	controller.PostUserEvents(w, req)

	assert.Equal(t, 400, w.Code)
}

func TestPostEventsMissingRBACScope(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)

	events := []map[string]any{
		{
			"name":       "test_event",
			"identifier": []map[string]any{{"source": "default", "external_id": "user_123"}},
		},
	}

	body, err := json.Marshal(events)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/users/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	controller.PostUserEvents(w, req)

	assert.Equal(t, 401, w.Code)
}

func TestPostEventsMissingProjectID(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)

	orgID, err := controller.mgmt.OrganizationsStore.CreateOrganization(t.Context(), "Test Org")
	require.NoError(t, err)

	events := []map[string]any{
		{
			"name":       "test_event",
			"identifier": []map[string]any{{"source": "default", "external_id": "user_123"}},
		},
	}

	body, err := json.Marshal(events)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/users/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	actor := rbac.NewActor(rbac.ActorAPIKey, uuid.New().String(),
		rbac.WithOrganizationID(orgID),
	)

	req = req.WithContext(rbac.WithActor(req.Context(), actor))
	w := httptest.NewRecorder()

	controller.PostUserEvents(w, req)

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
			"name":       "complex_event",
			"identifier": []map[string]any{{"source": "default", "external_id": "user_123"}},
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

	req := httptest.NewRequest("POST", "/api/client/users/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	actor := controller.actorContext(t, orgID, projectID)
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
	w := httptest.NewRecorder()

	controller.PostUserEvents(w, req)

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

	req := httptest.NewRequest("POST", "/api/client/users/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	actor := controller.actorContext(t, orgID, projectID)
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
	w := httptest.NewRecorder()

	controller.PostUserEvents(w, req)

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
				"identifier": []map[string]any{
					{"source": "default", "external_id": "user_123"},
				},
				"email":    "user@example.com",
				"timezone": "America/Chicago",
				"locale":   "en",
				"data": map[string]any{
					"first_name": "John",
					"last_name":  "Smith",
				},
			},
			statusCode: 200,
		},
		"identify with anonymous_id": {
			body: map[string]any{
				"identifier": []map[string]any{
					{"source": "anonymous", "external_id": "anon_abc"},
				},
				"email": "test@test.com",
			},
			statusCode: 200,
		},
		"identify with minimal data": {
			body: map[string]any{
				"identifier": []map[string]any{
					{"source": "default", "external_id": "user_456"},
				},
			},
			statusCode: 200,
		},
		"identify with phone": {
			body: map[string]any{
				"identifier": []map[string]any{
					{"source": "default", "external_id": "user_789"},
				},
				"phone":    "+1234567890",
				"timezone": "Europe/Amsterdam",
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

			req := httptest.NewRequest("POST", "/api/client/users", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			actor := controller.actorContext(t, orgID, projectID)
			req = req.WithContext(rbac.WithActor(req.Context(), actor))
			w := httptest.NewRecorder()

			controller.UpsertUserClient(w, req)

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
				"identifier": []map[string]any{},
				"email":      "test@test.com",
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

			req := httptest.NewRequest("POST", "/api/client/users", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			actor := controller.actorContext(t, orgID, projectID)
			req = req.WithContext(rbac.WithActor(req.Context(), actor))
			w := httptest.NewRecorder()

			controller.UpsertUserClient(w, req)

			assert.Equal(t, tc.statusCode, w.Code)
		})
	}
}

func TestClientIdentifyUserMissingRBACScope(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)

	body, err := json.Marshal(map[string]any{
		"identifier": []map[string]any{{"source": "default", "external_id": "user_123"}},
		"email":      "test@test.com",
	})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	controller.UpsertUserClient(w, req)

	assert.Equal(t, 401, w.Code)
}

func TestClientIdentifyUserMissingProjectID(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)

	orgID, err := controller.mgmt.OrganizationsStore.CreateOrganization(t.Context(), "Test Org")
	require.NoError(t, err)

	body, err := json.Marshal(map[string]any{
		"identifier": []map[string]any{{"source": "default", "external_id": "user_123"}},
		"email":      "test@test.com",
	})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	actor := rbac.NewActor(rbac.ActorAPIKey, uuid.New().String(),
		rbac.WithOrganizationID(orgID),
	)

	req = req.WithContext(rbac.WithActor(req.Context(), actor))
	w := httptest.NewRecorder()

	controller.UpsertUserClient(w, req)

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

	actor := controller.actorContext(t, orgID, projectID)

	// First identify call
	body1, err := json.Marshal(map[string]any{
		"identifier": []map[string]any{{"source": "default", "external_id": "user_123"}},
		"email":      "original@example.com",
		"timezone":   "America/New_York",
		"data": map[string]any{
			"first_name": "John",
		},
	})
	require.NoError(t, err)

	req1 := httptest.NewRequest("POST", "/api/client/users", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	req1 = req1.WithContext(rbac.WithActor(req1.Context(), actor))
	w1 := httptest.NewRecorder()

	controller.UpsertUserClient(w1, req1)

	assert.Equal(t, 200, w1.Code)

	var response1 map[string]any
	err = json.Unmarshal(w1.Body.Bytes(), &response1)
	require.NoError(t, err)
	userID1 := response1["id"]

	// Second identify call with updated data
	body2, err := json.Marshal(map[string]any{
		"identifier": []map[string]any{{"source": "default", "external_id": "user_123"}},
		"email":      "updated@example.com",
		"timezone":   "Europe/Amsterdam",
		"data": map[string]any{
			"first_name": "John",
			"last_name":  "Doe",
		},
	})
	require.NoError(t, err)

	req2 := httptest.NewRequest("POST", "/api/client/users", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2 = req2.WithContext(rbac.WithActor(req2.Context(), actor))
	w2 := httptest.NewRecorder()

	controller.UpsertUserClient(w2, req2)

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
		"identifier": []map[string]any{{"source": "default", "external_id": "user_123"}, {"source": "anonymous", "external_id": "anon_abc"}},
		"email":      "test@test.com",
	})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	actor := controller.actorContext(t, orgID, projectID)
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
	w := httptest.NewRecorder()

	controller.UpsertUserClient(w, req)

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
				"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
				"name":       "Acme Corp",
				"data": map[string]any{
					"industry": "technology",
					"size":     "enterprise",
				},
			},
			statusCode: 200,
		},
		"create organization with minimal data": {
			body: map[string]any{
				"identifier": []map[string]any{{"source": "default", "external_id": "org_456"}},
			},
			statusCode: 200,
		},
		"create organization with name only": {
			body: map[string]any{
				"identifier": []map[string]any{{"source": "default", "external_id": "org_789"}},
				"name":       "Simple Corp",
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

			actor := controller.actorContext(t, orgID, projectID)
			req = req.WithContext(rbac.WithActor(req.Context(), actor))
			w := httptest.NewRecorder()

			controller.UpsertOrganizationClient(w, req)

			assert.Equal(t, tc.statusCode, w.Code)
			if w.Code == 200 {
				var response map[string]any
				err = json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.NotEmpty(t, response["id"])
				reqIdentifiers := tc.body["identifier"].([]map[string]any)
				respIdentifiers := response["identifier"].([]any)
				require.Len(t, respIdentifiers, len(reqIdentifiers))
				respFirst := respIdentifiers[0].(map[string]any)
				assert.Equal(t, reqIdentifiers[0]["external_id"], respFirst["external_id"])
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

	actor := controller.actorContext(t, orgID, projectID)

	// First upsert - create
	body1, err := json.Marshal(map[string]any{
		"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
		"name":       "Original Name",
		"data": map[string]any{
			"plan": "basic",
		},
	})
	require.NoError(t, err)

	req1 := httptest.NewRequest("POST", "/api/client/organizations", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	req1 = req1.WithContext(rbac.WithActor(req1.Context(), actor))
	w1 := httptest.NewRecorder()

	controller.UpsertOrganizationClient(w1, req1)
	assert.Equal(t, 200, w1.Code)

	var response1 map[string]any
	err = json.Unmarshal(w1.Body.Bytes(), &response1)
	require.NoError(t, err)
	orgInternalID := response1["id"]

	// Second upsert - update
	body2, err := json.Marshal(map[string]any{
		"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
		"name":       "Updated Name",
		"data": map[string]any{
			"plan": "enterprise",
		},
	})
	require.NoError(t, err)

	req2 := httptest.NewRequest("POST", "/api/client/organizations", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2 = req2.WithContext(rbac.WithActor(req2.Context(), actor))
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
		"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
		"name":       "Test Org",
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
		"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
		"name":       "Test Org",
	})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/organizations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	actor := rbac.NewActor(rbac.ActorAPIKey, uuid.New().String(),
		rbac.WithOrganizationID(orgID),
	)

	req = req.WithContext(rbac.WithActor(req.Context(), actor))
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

	actor := controller.actorContext(t, orgID, projectID)
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
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
				"organization": map[string]any{
					"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
				},
				"user": map[string]any{
					"identifier": []map[string]any{{"source": "default", "external_id": "user_456"}},
				},
				"data": map[string]any{
					"role":       "admin",
					"department": "engineering",
				},
			},
			statusCode: 200,
		},
		"add user without data": {
			body: map[string]any{
				"organization": map[string]any{
					"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
				},
				"user": map[string]any{
					"identifier": []map[string]any{{"source": "default", "external_id": "user_789"}},
				},
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

			actor := controller.actorContext(t, orgID, projectID)

			// Create the subject organization first
			orgBody, err := json.Marshal(map[string]any{
				"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
				"name":       "Test Subject Org",
			})
			require.NoError(t, err)

			orgReq := httptest.NewRequest("POST", "/api/client/organizations", bytes.NewReader(orgBody))
			orgReq.Header.Set("Content-Type", "application/json")
			orgReq = orgReq.WithContext(rbac.WithActor(orgReq.Context(), actor))
			orgW := httptest.NewRecorder()
			controller.UpsertOrganizationClient(orgW, orgReq)
			require.Equal(t, 200, orgW.Code)

			// Create the user
			userObj := tc.body["user"].(map[string]any)
			userIdent := userObj["identifier"].([]map[string]any)
			userExternalID := userIdent[0]["external_id"].(string)
			userBody, err := json.Marshal(map[string]any{
				"identifier": []map[string]any{{"source": "default", "external_id": userExternalID}},
				"email":      userExternalID + "@example.com",
			})
			require.NoError(t, err)

			userReq := httptest.NewRequest("POST", "/api/client/users", bytes.NewReader(userBody))
			userReq.Header.Set("Content-Type", "application/json")
			userReq = userReq.WithContext(rbac.WithActor(userReq.Context(), actor))
			userW := httptest.NewRecorder()
			controller.UpsertUserClient(userW, userReq)
			require.Equal(t, 200, userW.Code)

			// Add user to organization
			body, err := json.Marshal(tc.body)
			require.NoError(t, err)

			req := httptest.NewRequest("POST", "/api/client/organizations/users", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(rbac.WithActor(req.Context(), actor))
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
		"organization": map[string]any{
			"identifier": []map[string]any{{"source": "default", "external_id": "nonexistent_org"}},
		},
		"user": map[string]any{
			"identifier": []map[string]any{{"source": "default", "external_id": "user_123"}},
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/organizations/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	actor := controller.actorContext(t, orgID, projectID)
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
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

	actor := controller.actorContext(t, orgID, projectID)

	// Create the subject organization first
	orgBody, err := json.Marshal(map[string]any{
		"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
		"name":       "Test Subject Org",
	})
	require.NoError(t, err)

	orgReq := httptest.NewRequest("POST", "/api/client/organizations", bytes.NewReader(orgBody))
	orgReq.Header.Set("Content-Type", "application/json")
	orgReq = orgReq.WithContext(rbac.WithActor(orgReq.Context(), actor))
	orgW := httptest.NewRecorder()
	controller.UpsertOrganizationClient(orgW, orgReq)
	require.Equal(t, 200, orgW.Code)

	body, err := json.Marshal(map[string]any{
		"organization": map[string]any{
			"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
		},
		"user": map[string]any{
			"identifier": []map[string]any{{"source": "default", "external_id": "nonexistent_user"}},
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/organizations/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
	w := httptest.NewRecorder()

	controller.AddOrganizationUserClient(w, req)

	assert.Equal(t, 404, w.Code)
}

func TestAddOrganizationUserClientMissingRBACScope(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)

	body, err := json.Marshal(map[string]any{
		"organization": map[string]any{
			"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
		},
		"user": map[string]any{
			"identifier": []map[string]any{{"source": "default", "external_id": "user_456"}},
		},
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

	actor := controller.actorContext(t, orgID, projectID)

	// Create the subject organization
	orgBody, err := json.Marshal(map[string]any{
		"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
		"name":       "Test Subject Org",
	})
	require.NoError(t, err)

	orgReq := httptest.NewRequest("POST", "/api/client/organizations", bytes.NewReader(orgBody))
	orgReq.Header.Set("Content-Type", "application/json")
	orgReq = orgReq.WithContext(rbac.WithActor(orgReq.Context(), actor))
	orgW := httptest.NewRecorder()
	controller.UpsertOrganizationClient(orgW, orgReq)
	require.Equal(t, 200, orgW.Code)

	// Create the user
	userBody, err := json.Marshal(map[string]any{
		"identifier": []map[string]any{{"source": "default", "external_id": "user_456"}},
		"email":      "user@example.com",
	})
	require.NoError(t, err)

	userReq := httptest.NewRequest("POST", "/api/client/users", bytes.NewReader(userBody))
	userReq.Header.Set("Content-Type", "application/json")
	userReq = userReq.WithContext(rbac.WithActor(userReq.Context(), actor))
	userW := httptest.NewRecorder()
	controller.UpsertUserClient(userW, userReq)
	require.Equal(t, 200, userW.Code)

	// Add user to organization
	addBody, err := json.Marshal(map[string]any{
		"organization": map[string]any{
			"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
		},
		"user": map[string]any{
			"identifier": []map[string]any{{"source": "default", "external_id": "user_456"}},
		},
	})
	require.NoError(t, err)

	addReq := httptest.NewRequest("POST", "/api/client/organizations/users", bytes.NewReader(addBody))
	addReq.Header.Set("Content-Type", "application/json")
	addReq = addReq.WithContext(rbac.WithActor(addReq.Context(), actor))
	addW := httptest.NewRecorder()
	controller.AddOrganizationUserClient(addW, addReq)
	require.Equal(t, 200, addW.Code)

	// Remove user from organization
	removeBody, err := json.Marshal(map[string]any{
		"organization": map[string]any{
			"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
		},
		"user": map[string]any{
			"identifier": []map[string]any{{"source": "default", "external_id": "user_456"}},
		},
	})
	require.NoError(t, err)

	removeReq := httptest.NewRequest("DELETE", "/api/client/organizations/users", bytes.NewReader(removeBody))
	removeReq.Header.Set("Content-Type", "application/json")
	removeReq = removeReq.WithContext(rbac.WithActor(removeReq.Context(), actor))
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
		"organization": map[string]any{
			"identifier": []map[string]any{{"source": "default", "external_id": "nonexistent_org"}},
		},
		"user": map[string]any{
			"identifier": []map[string]any{{"source": "default", "external_id": "user_123"}},
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest("DELETE", "/api/client/organizations/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	actor := controller.actorContext(t, orgID, projectID)
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
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

	actor := controller.actorContext(t, orgID, projectID)

	// Create the subject organization
	orgBody, err := json.Marshal(map[string]any{
		"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
		"name":       "Test Subject Org",
	})
	require.NoError(t, err)

	orgReq := httptest.NewRequest("POST", "/api/client/organizations", bytes.NewReader(orgBody))
	orgReq.Header.Set("Content-Type", "application/json")
	orgReq = orgReq.WithContext(rbac.WithActor(orgReq.Context(), actor))
	orgW := httptest.NewRecorder()
	controller.UpsertOrganizationClient(orgW, orgReq)
	require.Equal(t, 200, orgW.Code)

	body, err := json.Marshal(map[string]any{
		"organization": map[string]any{
			"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
		},
		"user": map[string]any{
			"identifier": []map[string]any{{"source": "default", "external_id": "nonexistent_user"}},
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest("DELETE", "/api/client/organizations/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
	w := httptest.NewRecorder()

	controller.RemoveOrganizationUserClient(w, req)

	assert.Equal(t, 404, w.Code)
}

func TestRemoveOrganizationUserClientMissingRBACScope(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)

	body, err := json.Marshal(map[string]any{
		"organization": map[string]any{
			"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
		},
		"user": map[string]any{
			"identifier": []map[string]any{{"source": "default", "external_id": "user_456"}},
		},
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
					"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
					"name":       "subscription_upgraded",
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
					"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
					"name":       "account_activated",
				},
			},
			statusCode: 202,
		},
		"multiple events": {
			events: []map[string]any{
				{
					"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
					"name":       "feature_enabled",
					"data": map[string]any{
						"feature": "advanced_analytics",
					},
				},
				{
					"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
					"name":       "user_invited",
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

			actor := controller.actorContext(t, orgID, projectID)

			// Create the subject organization first
			orgBody, err := json.Marshal(map[string]any{
				"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
				"name":       "Test Subject Org",
			})
			require.NoError(t, err)

			orgReq := httptest.NewRequest("POST", "/api/client/organizations", bytes.NewReader(orgBody))
			orgReq.Header.Set("Content-Type", "application/json")
			orgReq = orgReq.WithContext(rbac.WithActor(orgReq.Context(), actor))
			orgW := httptest.NewRecorder()
			controller.UpsertOrganizationClient(orgW, orgReq)
			require.Equal(t, 200, orgW.Code)

			// Post organization events
			body, err := json.Marshal(tc.events)
			require.NoError(t, err)

			req := httptest.NewRequest("POST", "/api/client/organizations/events", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(rbac.WithActor(req.Context(), actor))
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
			"identifier": []map[string]any{{"source": "default", "external_id": "nonexistent_org"}},
			"name":       "test_event",
		},
	}

	body, err := json.Marshal(events)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/organizations/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	actor := controller.actorContext(t, orgID, projectID)
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
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
			"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
			"name":       "test_event",
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
			"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
			"name":       "test_event",
		},
	}

	body, err := json.Marshal(events)
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/organizations/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	actor := rbac.NewActor(rbac.ActorAPIKey, uuid.New().String(),
		rbac.WithOrganizationID(orgID),
	)

	req = req.WithContext(rbac.WithActor(req.Context(), actor))
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

	actor := controller.actorContext(t, orgID, projectID)
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
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

	actor := controller.actorContext(t, orgID, projectID)

	// Create the subject organization first
	orgBody, err := json.Marshal(map[string]any{
		"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
		"name":       "Test Subject Org",
	})
	require.NoError(t, err)

	orgReq := httptest.NewRequest("POST", "/api/client/organizations", bytes.NewReader(orgBody))
	orgReq.Header.Set("Content-Type", "application/json")
	orgReq = orgReq.WithContext(rbac.WithActor(orgReq.Context(), actor))
	orgW := httptest.NewRecorder()
	controller.UpsertOrganizationClient(orgW, orgReq)
	require.Equal(t, 200, orgW.Code)

	events := []map[string]any{
		{
			"identifier": []map[string]any{{"source": "default", "external_id": "org_123"}},
			"name":       "complex_event",
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
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
	w := httptest.NewRecorder()

	controller.PostOrganizationEventsClient(w, req)

	assert.Equal(t, 202, w.Code)
}
